package hvsc

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/lhz/considerate/config"
)

const (
	SongLengthsFile = "DOCUMENTS/Songlengths.txt"
	TunesCacheFile  = "cache-tunes.json"
)

type SidHeader struct {
	MagicID     string
	Version     int
	DataOffset  uint16
	LoadAddress uint16
	InitAddress uint16
	PlayAddress uint16
	Songs       int
	StartSong   int
	Speed       uint32
	Name        string
	Author      string
	Released    string
	Flags       uint16
	StartPage   byte
	PageLength  byte
	Sid2Address byte
	Sid3Address byte
}

type SidTune struct {
	Path        string
	MD5         string
	NumSongs    int
	SongLengths []time.Duration
	Header      SidHeader
}

var Tunes = make([]SidTune, 0)
var NumTunes = 0

var header = make([]byte, 124)

// Read tunes data from cache file
func ReadTunesInfoCached() {
	if _, err := os.Stat(hvscPathTo(TunesCacheFile)); os.IsNotExist(err) {
		ReadTunesInfo()
		return
	}

	log.Print("Reading cached tunes info.")
	content, err := ioutil.ReadFile(hvscPathTo(TunesCacheFile))
	if err != nil {
		log.Fatal(err)
	}

	json.Unmarshal(content, &Tunes)
	NumTunes = len(Tunes)
}

// Build tunes data from .sid-files and various documents
func ReadTunesInfo() {
	file, err := os.Open(hvscPathTo(SongLengthsFile))
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	log.Print("Building tunes info cache.")
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line[0] == ';' {
			tune := SidTune{Path: line[2:]}
			tune.Header = ReadSidHeader(hvscPathTo(tune.Path))
			Tunes = append(Tunes, tune)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	NumTunes = len(Tunes)

	b, err := json.MarshalIndent(Tunes, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	jsonFile, err := os.Create(hvscPathTo(TunesCacheFile))
	if err != nil {
		log.Fatal(err)
	}
	defer jsonFile.Close()

	jsonFile.Write(b)
}

func ReadSidHeader(fileName string) SidHeader {
	file, err := os.Open(fileName)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	_, err = file.Read(header)
	if err != nil {
		log.Fatal(err)
	}

	enc := binary.BigEndian

	h := SidHeader{
		MagicID:     string(header[0:4]),
		Version:     int(enc.Uint16(header[4:])),
		DataOffset:  enc.Uint16(header[6:]),
		LoadAddress: enc.Uint16(header[8:]),
		InitAddress: enc.Uint16(header[10:]),
		PlayAddress: enc.Uint16(header[12:]),
		Songs:       int(enc.Uint16(header[14:])),
		StartSong:   int(enc.Uint16(header[16:])),
		Speed:       enc.Uint32(header[18:]),
		Name:        stringExtract(header[22:54]),
		Author:      stringExtract(header[54:86]),
		Released:    stringExtract(header[86:118]),
	}
	return h
}

func stringExtract(slice []byte) string {
	codePoints := make([]rune, len(slice))
	pos := 0
	for ; pos < len(slice) && slice[pos] != 0; pos++ {
		codePoints[pos] = rune(slice[pos])
	}
	return string(codePoints[:pos])
}

func hvscPathTo(filePath string) string {
	return fmt.Sprintf("%s/%s", config.Config.HvscPath, filePath)
}
