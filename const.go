// File for constants
// Roger Pau Monné - 2010
// Distributed under the terms of the GNU GPLv3

package main

const(
	FILE_PERM = 0666
	FOLDER_PERM = 0755
	PROTOCOL = "BitTorrent protocol"
	MAX_PEER_MSG = 130*1024
	ACTIVE_PEERS = 10
	INACTIVE_PEERS = 40
	TRACKER_ERR_INTERVAL = 20*NS_PER_S
	NS_PER_S = 1000000000
	KEEP_ALIVE_ROUND = 60*NS_PER_S
	KEEP_ALIVE_MSG = 120*NS_PER_S
	TIMEOUT = 6*60*NS_PER_S
	)

const (
	choke	= iota;
	unchoke;
	interested;
	uninterested;
	have;
	bitfield;
	request;
	piece;
	cancel;
	port;
	exit;
)
