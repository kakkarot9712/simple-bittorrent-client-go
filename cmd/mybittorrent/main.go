package main

import (
	// Uncomment this line to pass the first stage
	// "encoding/json"

	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	// Available if you need it!
)

func main() {
	// You can use print statements as follows for debugging, they'll be visible when running tests.
	// fmt.Println("Logs from your program will appear here!")

	command := os.Args[1]

	if command == "decode" {
		// Uncomment this block to pass the first stage

		bencodedValue := os.Args[2]

		decoded, err := decodeBencode(bencodedValue)
		if err != nil {
			fmt.Println(err)
			return
		}

		jsonOutput, _ := json.Marshal(decoded)
		fmt.Println(string(jsonOutput))
	} else if command == "info" {
		filePath := os.Args[2]
		t := torrent{}
		t.Open(filePath)
		hexHash := hex.EncodeToString(t.GetInfoHash())
		hashes := t.GetPieceHashes()

		fmt.Printf("Tracker URL: %v\nLength: %v\nInfo Hash: %v\nPiece Length: %v\nPiece Hashes:\n", t.Announce, t.Info.Length, hexHash, t.Info.PieceLength)
		for _, hash := range hashes {
			fmt.Println(hash)
		}
	} else if command == "peers" {
		filePath := os.Args[2]
		t := torrent{}
		t.Open(filePath)
		peer_id := GetPeerId()
		ar := t.AnnounceToServer(peer_id)
		peers := ar.GetPeers()
		for _, peer := range peers {
			fmt.Println(peer)
		}
	} else if command == "handshake" {
		file := os.Args[2]
		host := os.Args[3]
		t := torrent{}
		t.Open(file)
		peerId := GetPeerId()
		conn, pid := t.StartConn(host, peerId)
		fmt.Println("Peer ID:", pid)
		conn.Close()
	} else if command == "download_piece" {
		// outFolder := os.Args[3]
		// file := os.Args[4]
		// pieceIndex := os.Args[5]
		// t := torrent{}
		// t.Open(file)
		// pid := GetPeerId()
		// ar := t.AnnounceToServer(pid)
		// peers := ar.GetPeers()
		// conn, _ := t.StartConn(peers[0], pid)

		// TODO: Download One Piece
	} else {
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}
