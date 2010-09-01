// Handles a peer
// Roger Pau Monné - 2010
// Distributed under the terms of the GNU GPLv3

package main

import(
	"log"
	"os"
	"net"
	"time"
	"encoding/binary"
	"sync"
	"bytes"
	)

type Peer struct {
	addr, remote_peerId, our_peerId, infohash string
	numPieces int64
	wire *Wire
	bitfield *Bitfield
	our_bitfield *Bitfield
	in chan *message
	incoming chan *message // Exclusive channel, where peer receives messages and PeerMgr sends
	outgoing chan *message // Shared channel, peer sends messages and PeerMgr receives
	requests chan *PieceMgrRequest // Shared channel with the PieceMgr, used to request new pieces
	delete chan *message
	am_choking bool
	am_interested bool
	peer_choking bool
	peer_interested bool
	connected bool
	last bool
	received_keepalive int64
	writeQueue *PeerQueue
	mutex *sync.Mutex
	stats chan *PeerStatMsg
	//log *logger
	keepAlive *time.Ticker
	inFiles chan *FileStoreMsg
}

func NewPeer(addr, infohash, peerId string, outgoing chan *message, numPieces int64, requests chan *PieceMgrRequest, our_bitfield *Bitfield, stats chan *PeerStatMsg, inFiles chan *FileStoreMsg) (p *Peer, err os.Error) {
	p = new(Peer)
	p.mutex = new(sync.Mutex)
	p.addr = addr
	//p.log, err = NewLogger(p.addr)
	p.infohash = infohash
	p.our_peerId = peerId
	p.incoming = make(chan *message)
	p.in = make(chan *message)
	p.outgoing = outgoing
	p.inFiles = inFiles
	p.am_choking = true
	p.am_interested = false
	p.peer_choking = true
	p.peer_interested = false
	p.connected = false
	p.bitfield = NewBitfield(numPieces)
	p.our_bitfield = our_bitfield
	p.numPieces = numPieces
	p.requests = requests
	p.stats = stats
	p.delete = make(chan *message)
	// Start writting queue
	p.in = make(chan *message)
	p.keepAlive = time.NewTicker(KEEP_ALIVE_MSG)
	p.writeQueue = NewQueue(p.incoming, p.in, p.delete)
	go p.writeQueue.Run()
	return
}

func NewPeerFromConn(conn *net.Conn, infohash, peerId string, outgoing chan *message, numPieces int64, requests chan *PieceMgrRequest, our_bitfield *Bitfield, stats chan *PeerStatMsg, inFiles chan *FileStoreMsg) (p *Peer, err os.Error) {
	p = new(Peer)
	p.mutex = new(sync.Mutex)
	p.addr = conn.RemoteAddr().String()
	//p.log, err = NewLogger(p.addr)
	p.infohash = infohash
	p.our_peerId = peerId
	p.incoming = make(chan *message)
	p.in = make(chan *message)
	p.outgoing = outgoing
	p.inFiles = inFiles
	p.am_choking = true
	p.am_interested = false
	p.peer_choking = true
	p.peer_interested = false
	p.bitfield = NewBitfield(numPieces)
	p.our_bitfield = our_bitfield
	p.numPieces = numPieces
	p.requests = requests
	p.stats = stats
	p.delete = make(chan *message)
	// Start writting queue
	p.in = make(chan *message)
	p.keepAlive = time.NewTicker(KEEP_ALIVE_MSG)
	p.writeQueue = NewQueue(p.incoming, p.in, p.delete)
	p.wire, err = NewWire(p.infohash, p.our_peerId, *conn)
	go p.writeQueue.Run()
	return
}

func (p *Peer) preprocessMessage(msg *message) (skip bool, err os.Error) {
	if msg == nil {
		err = os.NewError("Nil message")
		return
	}
	switch msg.msgId {
		case unchoke:
			if !p.am_choking {
				// Avoid sending repeated unchoke messages
				skip = true
			} else {
				p.am_choking = false
			}
		case choke:
			if p.am_choking {
				// Avoid sending repeated choke messages
				skip = true
			} else {
				p.am_choking = true
				// Flush peer request queue
				p.incoming <- &message{length: 1, msgId: flush}
			}
		case interested:
			if p.am_interested {
				skip = true
			} else {
				p.am_interested = true
			}
		case uninterested:
			if !p.am_interested {
				skip = true
			} else {
				p.am_interested = false
			}
		case piece:
			// Read block from the disk and fill the request before sending
			fileMsg := new(FileStoreMsg)
			index := binary.BigEndian.Uint32(msg.payLoad[0:4])
			begin := binary.BigEndian.Uint32(msg.payLoad[4:8])
			length := binary.BigEndian.Uint32(msg.payLoad[8:12])
			buffer := make([]byte, 0, length+8)
			buffer = bytes.Add(buffer, msg.payLoad[0:8])
			fileMsg.Id = readat
			fileMsg.Index = int64(index)
			fileMsg.Begin = int64(begin)
			fileMsg.Bytes = buffer[8:length+8]
			fileMsg.Response = make(chan *FileStoreMsg)
			p.inFiles <- fileMsg
			fileMsg = <- fileMsg.Response
			if !fileMsg.Ok {
				log.Stderr(fileMsg.Err)
				break
			}
			msg.payLoad = buffer[0:length+8]
			msg.length = length + uint32(9)
	}
	return
}


func (p *Peer) PeerWriter() {
	// Create connection
	defer p.Close()
	var err os.Error
	if p.wire == nil {
		addrTCP, err := net.ResolveTCPAddr(p.addr)
		if err != nil {
			//p.log.Output(err, p.addr)
			return
		}
		conn, err := net.DialTCP("tcp4", nil, addrTCP)
		if err != nil {
			//p.log.Output(err, p.addr)
			return
		}
		/*err = conn.SetTimeout(TIMEOUT)
		if err != nil {
			//p.log.Output(err, p.addr)
			return
		}*/
		// Create the wire struct
		p.wire, err = NewWire(p.infohash, p.our_peerId, conn)
		if err != nil {
			return
		}
	}
	// Send handshake
	p.remote_peerId, err = p.wire.Handshake()
	if err != nil {
		//p.log.Output(err, p.addr)
		return
	}
	if p.remote_peerId == p.our_peerId {
		//p.log.Output("Local loopback")
		return
	}
	// Launch peer reader
	go p.PeerReader()
	// Send the have message
	our_bitfield := p.our_bitfield.Bytes()
	_, err = p.wire.WriteMsg(&message{length: uint32(1 + len(our_bitfield)), msgId: bitfield, payLoad: our_bitfield})
	if err != nil {
		//p.log.Output(err, p.addr)
		return
	}
	// Peer writer main bucle
	p.connected = true
	for !closed(p.in) {
		//p.log.Output("PeerWriter -> Waiting for message to send to", p.addr)
		select {
			// Wait for messages or send keep-alive
			case msg := <- p.in:
				skip, err := p.preprocessMessage(msg)
				if err != nil {
					return
				}
				if skip {
					continue
				}
				n, err := p.wire.WriteMsg(msg)
				if err != nil || n != int(4+msg.length) {
					//p.log.Output(err, p.addr, "written length:", n, "expected:", int(4 + msg.length))
					return
				}
				// Send message to StatMgr
				if msg.msgId == piece {
					statMsg := new(PeerStatMsg)
					statMsg.size_down = int64(len(msg.payLoad))
					statMsg.addr = p.addr
					p.stats <- statMsg
					//log.Stderr(statMsg)
				}
				// Reset ticker
				//close(p.keepAlive)
				p.keepAlive.Stop()
				p.keepAlive = time.NewTicker(KEEP_ALIVE_MSG)
				//p.log.Output("PeerWriter -> Finished sending message with id:", msg.msgId, "to", p.addr)
			case <- p.keepAlive.C:
				// Send keep-alive
				//p.log.Output("PeerWriter -> Sending Keep-Alive message to", p.addr)
				n, err := p.wire.WriteMsg(&message{length: 0})
				if err != nil || n != 4 {
					//p.log.Output(err, p.addr, "written length:", n, "expected:", 4)
					return
				}
				//p.log.Output("PeerWriter -> Finished sending Keep-Alive message to", p.addr)
		}
	}
}

func (p *Peer) PeerReader() {
	defer p.Close()
	for p.wire != nil {
		//p.log.Output("PeerReader -> Waiting for message from peer", p.addr)
		msg, _, err := p.wire.ReadMsg()
		if err != nil {
			//p.log.Output(err, p.addr)
			return
		}
		//p.log.Output("PeerReader -> Received message from", p.addr)
		if msg.length == 0 {
			p.received_keepalive = time.Seconds()
		} else {
			if msg.msgId == piece {
				statMsg := new(PeerStatMsg)
				statMsg.size_up = int64(len(msg.payLoad))
				statMsg.addr = p.addr
				p.stats <- statMsg
			}
			err := p.ProcessMessage(msg)
			if err != nil {
				//p.log.Output(err, p.addr)
				return
			}
		}
		//p.log.Output("PeerReader -> Finished processing message fromr", p.addr)
	}
}

func (p *Peer) ProcessMessage(msg *message) (err os.Error){
	//p.log.Output("Processing message with id:", msg.msgId)
	switch msg.msgId {
		case choke:
			// Choke peer
			p.peer_choking = true
			//p.log.Output("Peer", p.addr, "choked")
			// If choked, clear request list
			//p.log.Output("Cleaning request list")
			p.requests <- &PieceMgrRequest{msg: &message{length: 1, msgId: exit, addr: []string{p.addr}}}
			//p.log.Output("Finished cleaning")
		case unchoke:
			// Unchoke peer
			p.peer_choking = false
			//log.Stderr("Peer", p.addr, "unchoked")
			// Check if we are still interested on this peer
			p.CheckInterested()
			// Notice PieceMgr of the unchoke
			p.TryToRequestPiece()
		case interested:
			// Mark peer as interested
			p.peer_interested = true
			//log.Stderr("Peer", p.addr, "interested")
		case uninterested:
			// Mark peer as uninterested
			p.peer_interested = false
			//log.Stderr("Peer", p.addr, "uninterested")
		case have:
			// Update peer bitfield
			p.bitfield.Set(int64(binary.BigEndian.Uint32(msg.payLoad)))
			if p.our_bitfield.Completed() && p.bitfield.Completed() {
				err = os.NewError("Peer not useful")
				return
			}
			p.CheckInterested()
			//log.Stderr("Peer", p.addr, "have")
			// If we are unchoked notice PieceMgr of the new piece
			p.TryToRequestPiece()
		case bitfield:
			// Set peer bitfield
			//log.Stderr(msg)
			p.bitfield, err = NewBitfieldFromBytes(p.numPieces, msg.payLoad)
			if err != nil {
				return os.NewError("Invalid bitfield")
			}
			if p.our_bitfield.Completed() && p.bitfield.Completed() {
				err = os.NewError("Peer not useful")
				return
			}
			p.CheckInterested()
			p.TryToRequestPiece()
			//log.Stderr("Peer", p.addr, "bitfield")
		case request:
			// Peer requests a block
			//log.Stderr("Peer", p.addr, "requests a block")
			if !p.am_choking {
				p.requests <- &PieceMgrRequest{msg: msg, response: p.incoming}
			}
		case piece:
			//p.log.Output("Received piece, sending to pieceMgr")
			p.requests <- &PieceMgrRequest{msg: msg}
			// Check if the peer is still interesting
			//p.log.Output("Checking if interesting")
			p.CheckInterested()
			// Try to request another block
			//p.log.Output("Trying to request a new piece")
			p.TryToRequestPiece()
			//p.log.Output("Finished requesting new piece")
		case cancel:
			// Send the message to the sending queue to delete the "piece" message
			p.delete <- msg
		case port:
			// DHT stuff
		default:
			//p.log.Output("Unknown message")
			return os.NewError("Unknown message")
	}
	//p.log.Output("Finished processing")
	return
}

func (p *Peer) CheckInterested() {
	if p.am_interested && !p.our_bitfield.HasMorePieces(p.bitfield) {
		//p.am_interested = false
		p.incoming <- &message{length: 1, msgId: uninterested}
		//log.Stderr("Peer", p.addr, "marked as uninteresting")
		return
	}
	if !p.am_interested && p.our_bitfield.HasMorePieces(p.bitfield) {
		//p.am_interested = true
		p.incoming <- &message{length: 1, msgId: interested}
		//log.Stderr("Peer", p.addr, "marked as interesting")
		return
	}
}

func (p *Peer) TryToRequestPiece() {
	if p.am_interested && !p.peer_choking && !p.our_bitfield.Completed() {
		//p.log.Output("Sending request for new piece")
		p.requests <- &PieceMgrRequest{bitfield: p.bitfield, response: p.incoming, our_addr: p.addr, msg: &message{length: 1, msgId: our_request}}
		//p.log.Output("Finished sending request for new piece")
	}
}

func (p *Peer) Close() {
	//p.log.Output("Finishing peer")
	p.mutex.Lock()
	defer p.mutex.Unlock()
	//p.log.Output("Sending message to peerMgr")
	p.outgoing <- &message{length: 1, msgId: exit, addr: []string{p.addr}}
	//p.log.Output("Finished sending message")
	//p.log.Output("Sending message to pieceMgr")
	p.requests <- &PieceMgrRequest{msg: &message{length: 1, msgId: exit, addr: []string{p.addr}}}
	//p.log.Output("Finished sending message")
	// Sending message to Stats
	p.stats <- &PeerStatMsg{size_up: 0, size_down: 0, addr: p.addr}
	// Finished
	close(p.incoming)
	close(p.in)
	close(p.delete)
	p.keepAlive.Stop()
	// Here we could have a crash
	if p.wire != nil {
		p.wire.Close()
	}
	if p.last {
		p.wire = nil
		p.bitfield = nil
		p.our_bitfield = nil
		p.writeQueue = nil
		p.keepAlive = nil
		//p.log.Output("Removed all info")
	} else {
		p.last = true
	} 
	//p.log.Output("Finished closing peer")
}
