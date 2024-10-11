package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"time"
)

type Client struct {
	TorrentFile   string
	Torrent       torrent
	Unchocked     bool
	conn          net.Conn
	PeerId        string
	Peers         []string
	BitfieldBytes []byte
}

func (c *Client) Initialize() {
	t := torrent{}
	t.Open(c.TorrentFile)
	pid := GetPeerId()
	c.Torrent = t
	c.PeerId = pid
}

func (c *Client) GetPeersFromAnnounceServer() {
	ar := c.Torrent.AnnounceToServer(c.PeerId)
	c.Peers = ar.GetPeers()
}

func (c *Client) ConnectToPeer() {
	conn, err := net.DialTimeout("tcp", c.Peers[0], 3*time.Second)
	if err != nil {
		log.Fatal("Connection to peer host failed! aborting")
	}
	handshake := c.Torrent.GetHandshakeString(c.PeerId)
	_, err = conn.Write(handshake)
	if err != nil {
		log.Fatal("connection write failed!")
	}
	var buff = make([]byte, 512)
	for {
		// size, err := io.Copy(&buff, conn)
		size, err := conn.Read(buff)
		if err != nil {
			panic(err)
		}
		if size == 0 {
			continue
		}
		msg := buff
		if bytes.HasPrefix(msg, []byte{19, 66, 105, 116, 84, 111, 114, 114, 101, 110, 116, 32, 112, 114, 111, 116, 111, 99, 111, 108}) {
			fmt.Println("Connection is Valid")
			continue
		}
		length := binary.BigEndian.Uint32(msg[:4])
		if length == 0 {
			fmt.Println("Keep-Alive received")
			continue
		}
		messageId := message(msg[4])
		switch messageId {
		case Bitfields:
			fmt.Println("Bitfields Received")
			data := msg[5:size]
			fieldBytes := data[:length-1]
			c.BitfieldBytes = fieldBytes
			c.conn = conn
			return
		default:
			fmt.Println(msg[:size])
			fmt.Println("Unhandled initial response", messageId)
		}
		// buff = make([]byte, 512)
		// buff.Reset()
	}
}

func (c *Client) GetBlocksPerPieces() (int, int) {
	fullPiecesNum := c.Torrent.Info.PieceLength / uint(BLOCK_SIZE)
	lastBlockLength := c.Torrent.Info.PieceLength % uint(BLOCK_SIZE)
	if lastBlockLength > 0 {
		return int(fullPiecesNum) + 1, int(lastBlockLength)
	}
	return int(fullPiecesNum), int(lastBlockLength)
}

func (client *Client) DownloadPiece() {
	numPieces, lastPieceLength := client.GetBlocksPerPieces()
	for i := 0; i <= numPieces; i++ {
		payloadData := []byte{}
		payloadData = append(payloadData, byte(Request))
		pieceIndexBytes := make([]byte, 4)
		piecieOffsetBytes := make([]byte, 4)
		payloadLenghtBytes := make([]byte, 4)
		pieceLengthBytes := make([]byte, 4)
		binary.BigEndian.PutUint32(pieceIndexBytes, uint32(i))
		if i == numPieces {
			// Last Piece
			binary.BigEndian.PutUint32(pieceLengthBytes, uint32(lastPieceLength))
		} else {
			binary.BigEndian.PutUint32(pieceLengthBytes, BLOCK_SIZE)
		}
		binary.BigEndian.PutUint32(piecieOffsetBytes, uint32(i)*BLOCK_SIZE)
		payloadData = append(payloadData, pieceIndexBytes...)
		payloadData = append(payloadData, piecieOffsetBytes...)
		binary.BigEndian.PutUint32(payloadLenghtBytes, uint32(len(payloadData)))
		payloadData = append(payloadData, pieceLengthBytes...)
		// uintBytes := make([]byte, 8)

		// Request, Index, Offset, Length
		// payload := []byte{6, 0x0, 0x0, 0x40, 0x00}
		// payload = []byte{0, 0, 0, byte(len(payload)), byte(Request), 0x0, 0x0, 0x40, 0x00}
		// binary.BigEndian.PutUint32(lenghBytes, uint32(len(payload)))
		payload := []byte{}
		payload = append(payload, payloadLenghtBytes...)
		payload = append(payload, payloadData...)
		fmt.Println(payload)
		client.conn.Write(payload)
	}
}
