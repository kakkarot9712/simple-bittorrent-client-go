package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/jackpal/bencode-go"
)

type torrent struct {
	Announce string `bencode:"announce"`
	Info     struct {
		Length      uint   `bencode:"length"`
		Name        string `bencode:"name"`
		PieceLength uint   `bencode:"piece length"`
		Pieces      string `bencode:"pieces"`
	} `bencode:"info"`
}

type message uint8

const (
	Choke message = iota
	Unchoke
	Interested
	NotInterested
	Have
	Bitfields
	Request
	Piece
	Cancel
)

const BLOCK_SIZE uint32 = 16 * 1024

// Example:
// - 5:hello -> hello
// - 10:hello12345 -> hello12345
func decodeBencode(bencodedString string) (interface{}, error) {
	// startChars := []string{"l", "i"}
	if unicode.IsDigit(rune(bencodedString[0])) {
		var firstColonIndex int

		for i := 0; i < len(bencodedString); i++ {
			if bencodedString[i] == ':' {
				firstColonIndex = i
				break
			}
		}

		lengthStr := bencodedString[:firstColonIndex]

		length, err := strconv.Atoi(lengthStr)
		if err != nil {
			return "", err
		}

		return bencodedString[firstColonIndex+1 : firstColonIndex+1+length], nil
	} else if string(bencodedString[0]) == "i" {
		lastIndex := len(bencodedString) - 1
		lastChar := bencodedString[lastIndex]
		if string(lastChar) != "e" {
			return 0, errors.New("illegal end detected. want 'e' found '" + string(lastChar) + "'")
		}
		num, err := strconv.Atoi(bencodedString[1:lastIndex])
		if err != nil {
			return "", errors.New("invalid data detected")
		}
		return num, nil
		// l5:helloi52ee
	} else {
		var in bytes.Buffer
		_, err := in.Write([]byte(bencodedString))
		if err != nil {
			return "", errors.New("buffer write failed")
		}
		data, err := bencode.Decode(&in)
		if err != nil {
			return "", errors.New("invalid data detected")
		}

		return data, nil
	}
}

func (t *torrent) GetInfoHash() []byte {
	var out bytes.Buffer
	err := bencode.Marshal(&out, t.Info)
	if err != nil {
		log.Fatal("Encode failed!: ", err)
	}
	sha := sha1.New()
	sha.Write(out.Bytes())
	hash := sha.Sum(nil)
	return hash
}

func (t *torrent) GetPieceHashes() []string {
	rawHashes := []byte(t.Info.Pieces)
	if len(rawHashes)%20 != 0 {
		log.Fatal("Incomplete piece hashes detected! torrent file is probably corrupt")
	}
	totalHashes := len(rawHashes) / 20
	hashes := []string{}
	hashesCollected := 0
	for {
		hashes = append(hashes, hex.EncodeToString(rawHashes[hashesCollected*20:(hashesCollected*20)+20]))
		hashesCollected++
		if hashesCollected == totalHashes {
			break
		}
	}
	return hashes
}

func (t *torrent) GetAnnounceUrl(peer_id string, port string, uploaded string, downloaded string, left string) string {
	compact := "1"
	infoHash := t.GetInfoHash()
	announceUrl, err := url.Parse(t.Announce)
	if err != nil {
		log.Fatal("URL parse failed")
	}
	query := announceUrl.Query()
	query.Set("peer_id", peer_id)
	query.Set("port", port)
	query.Set("uploaded", uploaded)
	query.Set("downloaded", downloaded)
	query.Set("left", left)
	query.Set("compact", compact)
	query.Set("info_hash", string(infoHash))
	announceUrl.RawQuery = query.Encode()
	return announceUrl.String()
}

func (t *torrent) Open(filePath string) {
	buff, err := os.ReadFile(filePath)
	if err != nil {
		log.Fatal("File read failed!: ", err)
	}
	var in bytes.Buffer
	in.Write(buff)
	err = bencode.Unmarshal(&in, t)
	if err != nil {
		log.Fatal("Decode failed!: ", err)
	}
}

func (t *torrent) GetHandshakeString(peer_id string) (handshake []byte) {
	infoHash := t.GetInfoHash()
	handshake = []byte{}
	handshake = append(handshake, []byte("\x13BitTorrent protocol")...)
	handshake = append(handshake, make([]byte, 8)...)
	handshake = append(handshake, infoHash...)
	handshake = append(handshake, []byte(peer_id)...)
	return
}

// 1(19) + 19 + 8 + 20 + 20
func ParseHandshakeString(hs []byte) []byte {
	if string(hs[1:20]) != "BitTorrent protocol" {
		log.Fatal("Unsupported specification")
	}
	return hs[48:68]
}

func GetPeerId() (pid string) {
	letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	b := make([]rune, 20)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	pid = string(b)
	return
}

type announceResp struct {
	Interval uint   `bencode:"interval"`
	Peers    string `bencode:"peers"`
}

func (t *torrent) AnnounceToServer(peer_id string) announceResp {
	port := "6881"
	uploaded := "0"
	downloaded := "0"
	left := strconv.Itoa(int(t.Info.Length))
	announceUrl := t.GetAnnounceUrl(peer_id, port, uploaded, downloaded, left)
	res, err := http.Get(announceUrl)
	if err != nil {
		log.Fatal("GET Request failed: ", err)
	}
	ar := announceResp{}
	err = bencode.Unmarshal(res.Body, &ar)
	if err != nil {
		log.Fatal("Bencode Decode failed: ", err)
	}
	return ar
}

func (r *announceResp) GetPeers() []string {
	p := []byte(r.Peers)
	if len(p)%6 != 0 {
		log.Fatal("Invalid peers structure found! ensure you are requesting peer data in compact format.")
	}
	peersCount := len(p) / 6
	peersExtracted := 0
	peers := []string{}

	for {
		peer := p[peersExtracted*6 : (peersExtracted*6)+6]
		ipParts := []string{}
		for _, b := range peer[:4] {
			ipParts = append(ipParts, strconv.Itoa(int(b)))
		}
		ip := strings.Join(ipParts, ".")
		port := strconv.Itoa(int(binary.BigEndian.Uint16([]byte(peer[4:6]))))
		// if err != nil {
		// 	log.Fatal("port is invalid! torrent is probably corrupt")
		// }
		peers = append(peers, ip+":"+port)
		peersExtracted++
		if peersCount == peersExtracted {
			break
		}
	}
	return peers
}

func (t *torrent) StartConn(host string, peer_id string) (net.Conn, string) {
	conn, err := net.DialTimeout("tcp", host, 3*time.Second)
	if err != nil {
		log.Fatal("Connection to peer host failed! aborting")
	}
	handshake := t.GetHandshakeString(peer_id)
	_, err = conn.Write(handshake)
	if err != nil {
		log.Fatal("connection write failed!")
	}
	for {
		buff := make([]byte, 70)
		n, err := conn.Read(buff)
		if err != nil {
			log.Fatal("read failed! aborting...")
		}
		if n > 0 {
			return conn, hex.EncodeToString(ParseHandshakeString(buff))
		}
	}
}
