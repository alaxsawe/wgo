wgo - Simple BitTorrent client in Go
==========

Roger Pau Monné (2010 - 2011)

Introduction
------------

This project is based on the previous work of jackpal, Taipei-Torrent:
http://github.com/jackpal/Taipei-Torrent

Since Go is (or should become) a easy to use concurrent system programming
language I've decided to use it to develop a simple BitTorrent client. Some
of the functions are from the Taipei-Torrent project, and others are from
the gobit implementation found here:
http://github.com/jessta/gobit

Installation
------------

Simply run:

	gomake

Tests
-----

To run all the tests:
	
	gomake test

If you just want to run a single test, enter the corresponding folder and run gomake test.

Usage
-----

wgo is still in a VERY early phase, but you can try it, here are the flags:

	./wgo -torrent="path.to.torrent" -folder="/where/to/create/files" -procs=2 -port="6868" -up_limit=20 -down_limit=100

The up_limit and down_limit options are to limit the maximum upload/download,
and should be specified in KB/s. If ommited or set to 0, no limit is applied.

The procs option reflects the maximum number of processes the program can
use, this is almost only used when checking the hash, and can mean a big
improvement in the time needed to check the hash of a torrent. If you have
more than one processor, don't hesitate to set this to your number of processors,
or your number of processors minus one.

Other options are self explaining I think.

Source code Hierarchy
---------------------

Since this client aims to make heavy use of concurrency, the approach I've
taken is the same as Haskell-Torrent, this is just a copy of the module
description made by Jesper Louis Andersen

   - **Process**: Process definitions for the different processes comprising Haskell Torrent
      - **ChokeMgr**: Manages choking and unchoking of peers, based upon the current speed of the peer
        and its current state. Global for multiple torrents.
      - **Console**: Simple console process. Only responds to 'quit' at the moment.
      - **Files**: Process managing the file system.
      - **Listen**: Not used at the moment. Step towards listening sockets.
      - **Peer**: Several process definitions for handling peers. Two for sending, one for receiving
        and one for controlling the peer and handle the state.
      - **PeerMgr**: Management of a set of peers for a single torrent.
      - **PieceMgr**: Keeps track of what pieces have been downloaded and what are missing. Also hands
        out blocks for downloading to the peers.
      - **Status**: Keeps track of uploaded/downloaded/left bytes for a single torrent. Could be globalized.
      - **Timer**: Timer events.
      - **Limiter**: Limits the maximum upload and download speed of the program.
      - **Tracker**: Communication with the tracker.

   - **Protocol**: Modules for interacting with the various bittorrent protocols.
      - **Wire**: The protocol used for communication between peers.

   - **Top Level**:
      - **Const**: Several fine-tunning options, untill we are able to read them from a configuration file
      - **Torrent**: Various helpers and types for Torrents.
      - **Test**: Currently it holds the main part of the program

There's a nice graph that shows the proccess comunications:
	http://jlouisramblings.blogspot.com/2010/01/thoughts-on-process-hierarchies-in.html
	http://jlouisramblings.blogspot.com/2009/12/concurrency-bittorrent-clients-and.html
	http://jlouisramblings.blogspot.com/2009/12/on-peer-processes.html


