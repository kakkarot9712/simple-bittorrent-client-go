package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"os"
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

func (c *Client) GetBlocksPerPieces() int {
	fullPiecesNum := c.Torrent.Info.PieceLength / uint(BLOCK_SIZE)
	// lastBlockLength := c.Torrent.Info.PieceLength % uint(BLOCK_SIZE)
	return int(fullPiecesNum)
}

func (client *Client) DownloadPiece(pieceIndex int, outDir string) bool {
	conn := client.conn
	activeDownloads := 0
	numBlocks := client.GetBlocksPerPieces()
	fileLength := client.Torrent.Info.Length
	numPieces := fileLength / client.Torrent.Info.PieceLength
	lastPieceLength := fileLength % client.Torrent.Info.PieceLength
	if lastPieceLength != 0 {
		numPieces++
	}
	lastBlockLength := 0
	if pieceIndex == int(numPieces)-1 {
		numBlocks = int(lastPieceLength) / int(BLOCK_SIZE)
		lastBlockLength = int(lastPieceLength) % int(BLOCK_SIZE)
		if lastBlockLength != 0 {
			numBlocks++
		}
	}
	blockIndex := 0
	pieceDownloaded := false
	go func() {
		buff := make([]byte, BLOCK_SIZE+1000)
		var blockBuff bytes.Buffer
		blocksReceived := 0
		for {
			size, err := conn.Read(buff)
			if err != nil {
				panic(err)
			}
			if size > 0 {
				length := binary.BigEndian.Uint32(buff[:4])
				if length == 0 {
					fmt.Println("Keep-Alive received")
					continue
				}
				messageId := message(buff[4])
				switch messageId {
				case Choke:
					client.Unchocked = false
					return
				default:
					blockBuff.Write(buff[:size])
				}
				if blockBuff.Len() >= (blocksReceived+1)*(int(BLOCK_SIZE)+13) {
					activeDownloads--
					blocksReceived++
				}
				totalLength := int(client.Torrent.Info.PieceLength) + (numBlocks * 13)
				if lastPieceLength != 0 && pieceIndex == int(numPieces)-1 {
					totalLength = int(lastPieceLength) + (numBlocks * 13)
				}
				if blockBuff.Len() == totalLength {
					fmt.Println("ALL BLOCKS RECEIVED")
					var fileBuff bytes.Buffer
					file, err := os.OpenFile(outDir, os.O_CREATE|os.O_WRONLY|os.O_APPEND, os.ModePerm)
					if err != nil {
						panic(err)
					}
					defer file.Close()
					for i := 0; i < numBlocks; i++ {
						buff := make([]byte, BLOCK_SIZE+13)
						n, err := blockBuff.Read(buff)
						if err != nil {
							panic(err)
						}
						fileBuff.Write(buff[13:n])
					}
					// Validate Piece by using hash
					expectedHashHex := client.Torrent.GetPieceHashes()[pieceIndex]
					actualHash := sha1.New()
					actualHash.Write(fileBuff.Bytes())
					actualHex := hex.EncodeToString(actualHash.Sum(nil))
					if actualHex == expectedHashHex {
						fmt.Println("Checksum Verified, writing piece to disk")
						_, err = file.Write(fileBuff.Bytes())
						if err != nil {
							panic(err)
						}
					} else {
						log.Panic("piece checksum validation failed! aborting")
					}
					pieceDownloaded = true
					blockBuff.Reset()
					return
				}
			}
		}

	}()
	for !pieceDownloaded {
		if activeDownloads < 4 && blockIndex < numBlocks {
			payloadData := []byte{}
			payloadData = append(payloadData, byte(Request))
			// fmt.Println(numBlocks, numPieces, pieceIndex, lastPieceLength, lastBlockLength, blockIndex, "NP")
			pieceIndexBytes := make([]byte, 4)
			blockOffsetBytes := make([]byte, 4)
			binary.BigEndian.PutUint32(pieceIndexBytes, uint32(pieceIndex))
			binary.BigEndian.PutUint32(blockOffsetBytes, uint32(blockIndex)*BLOCK_SIZE)
			payloadData = append(payloadData, pieceIndexBytes...)
			payloadData = append(payloadData, blockOffsetBytes...)
			payloadLenghtBytes := make([]byte, 4)
			binary.BigEndian.PutUint32(payloadLenghtBytes, uint32(len(payloadData)))
			blockLengthBytes := make([]byte, 4)
			if blockIndex == numBlocks-1 && lastPieceLength != 0 && pieceIndex == int(numPieces)-1 {
				// fmt.Println("LAST BLP", blockIndex, lastBlockLength)
				binary.BigEndian.PutUint32(blockLengthBytes, uint32(lastBlockLength))
			} else {
				binary.BigEndian.PutUint32(blockLengthBytes, BLOCK_SIZE)
			}
			payloadData = append(payloadData, blockLengthBytes...)
			payload := []byte{}
			payload = append(payload, payloadLenghtBytes...)
			payload = append(payload, payloadData...)
			_, err := conn.Write(payload)
			if err == nil {
				// fmt.Println("req sent", blockIndex)
				activeDownloads++
				blockIndex++
			}
		}
	}
	return true
}
