package main

import (
	// Uncomment this line to pass the first stage
	// "encoding/json"

	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
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
		file := os.Args[4]
		client := Client{TorrentFile: file}
		client.Initialize()
		client.GetPeersFromAnnounceServer()
		// pieceIndex := os.Args[5]
		client.ConnectToPeer()
		conn := client.conn
		defer conn.Close()
		_, err := conn.Write([]byte{0, 0, 0, 1, byte(Interested)})
		if err != nil {
			panic(err)
		}
		buff := make([]byte, 512)
		for {
			size, err := conn.Read(buff)
			if err != nil {
				log.Panic(err)
			}
			if size == 0 {
				continue
			}
			length := binary.BigEndian.Uint32(buff[:4])
			if length == 0 {
				fmt.Println("Keep-Alive received")
				continue
			}
			messageId := message(buff[4])
			switch messageId {
			case Choke:
				fmt.Println("I am chocked")
				client.Unchocked = false
			case Piece:
				fmt.Println("Piece recived")
				fmt.Println(buff[:size], "PBUFF")
			case Unchoke:
				fmt.Println("I am unchocked")
				client.Unchocked = true

				// uintBytes := make([]byte, 8)

				// Request, Index, Offset
				// payload := []byte{6, 0x0, 0x0, 0x40, 0x00}
				// payload = []byte{0, 0, 0, byte(len(payload)), byte(Request), 0x0, 0x0, 0x40, 0x00}
				// binary.BigEndian.PutUint32(lenghBytes, uint32(len(payload)))
				conn.Write([]byte{0x00, 0x00, 0x00, 0x0D, 0x06, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x40, 0x00})
			default:
				fmt.Println("Unhandled response", messageId)
			}
			// buff = make([]byte, 512)
		}
	} else {
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}
