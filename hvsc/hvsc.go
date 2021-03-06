package hvsc

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/lhz/sidpicker/config"
	"github.com/lhz/sidpicker/csdb"
)

const (
	SongLengthsFile = "DOCUMENTS/Songlengths.txt"
	TunesIndexFile  = "tunes.json.gz"
	DefaultTitle    = "<?>"
)

var Release int
var Tunes = make([]SidTune, 0)
var NumTunes = 0

var header = make([]byte, 124)

func TuneIndexByPath(path string) int {
	for i, tune := range Tunes {
		if tune.Path == path {
			return i
		}
	}
	return -1
}

// Read tunes data from index file
func ReadTunesIndex() {
	detectRelease()

	if _, err := os.Stat(tunesIndexPath()); os.IsNotExist(err) {
		DownloadTunesIndex()
		ReadTunesIndex()
		return
	}

	//log.Printf("Reading tunes index from %q", tunesIndexPath())
	dataGzip, err := ioutil.ReadFile(tunesIndexPath())
	if err != nil {
		log.Fatal(err)
	}
	r, err := gzip.NewReader(bytes.NewBuffer(dataGzip))
	if err != nil {
		log.Fatal(err)
	}

	json.NewDecoder(r).Decode(&Tunes)
	NumTunes = len(Tunes)

	addDefaults()

	FilterAll()
}

// Download tunes index from the website
func DownloadTunesIndex() {
	url := fmt.Sprintf("https://github.com/lhz/sidtune-index/raw/master/hvsc-%d/tunes.json.gz", Release)
	fmt.Printf("Downloading index of tunes and releases.\n")
	response, err := http.Get(url)
	if err != nil {
		log.Fatalf("Error while downloading %q: %s", url, err)
	}
	if response.StatusCode >= 400 {
		log.Fatalf("Error while downloading %q: %s", url, response.Status)
	}
	defer response.Body.Close()

	output, err := os.Create(tunesIndexPath())
	if err != nil {
		log.Fatal("Error while creating file ", tunesIndexPath(), " - ", err)
	}
	defer output.Close()

	length, err := io.Copy(output, response.Body)
	if err != nil {
		log.Fatalf("Error while downloading %q: %s", url, err)
	}

	fmt.Printf("%.2fMB downloaded.\n", float64(length)/1000000)
}

// Build tunes data from .sid-files and various documents
func BuildTunesIndex() {
	file, err := os.Open(hvscPathTo(SongLengthsFile))
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	lr := regexp.MustCompile("[0-9]{1,2}:[0-9]{2}")

	log.Print("Building tunes index.")
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line[0] == ';' {
			tune := SidTune{Index: len(Tunes), Path: line[2:]}
			tune.Header = ReadSidHeader(hvscPathTo(tune.Path))
			tune.SongLengths = make([]time.Duration, tune.Header.Songs)
			tune.YearMin = tune.CalcYearMin()
			tune.YearMax = tune.CalcYearMax()
			Tunes = append(Tunes, tune)
		} else {
			lengths := lr.FindAllString(line, -1)
			for i, l := range lengths {
				Tunes[len(Tunes)-1].SongLengths[i] = parseSongLength(l)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	NumTunes = len(Tunes)
	FilterAll()

	readSTIL()
	readReleases()

	removeDefaults()

	dataJson, err := json.MarshalIndent(Tunes, "", "  ")
	if err != nil {
		log.Fatal(err)
	}

	var dataGzip bytes.Buffer
	w := gzip.NewWriter(&dataGzip)
	_, err = w.Write(dataJson)
	if err != nil {
		log.Fatal(err)
	}
	w.Close()

	err = ioutil.WriteFile(tunesIndexPath(), dataGzip.Bytes(), 0644)
	if err != nil {
		log.Fatal(err)
	}
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
		Flags:       enc.Uint16(header[118:]),
		StartPage:   header[120],
		PageLength:  header[121],
	}
	if header[122] > 0 {
		h.Sid2Address = uint16(header[122])*16 + 0xD000
	}
	if header[123] > 0 {
		h.Sid3Address = uint16(header[123])*16 + 0xD000
	}
	return h
}

func parseYear(value string, defVal int) int {
	year, err := strconv.Atoi(value)
	if err != nil {
		return defVal
	}
	if year < 100 {
		if year < 70 {
			year += 2000
		} else {
			year += 1900
		}
	}
	return year
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
	return fmt.Sprintf("%s/%s", config.Config.HvscBase, filePath)
}

func parseSongLength(value string) time.Duration {
	parts := strings.Split(value, ":")
	dur, err := time.ParseDuration(fmt.Sprintf("%sm%ss", parts[0], parts[1]))
	if err != nil {
		return 0
	}
	return dur
}

// Set default tune/header fields to empty values to reduce marshalling size
func removeDefaults() {
	for i, tune := range Tunes {
		if tune.YearMax == tune.YearMin {
			tune.YearMax = 0
		}
		if tune.Header.MagicID == "PSID" {
			tune.Header.MagicID = ""
		}
		if tune.Header.Version == 2 {
			tune.Header.Version = 0
		}
		if tune.Header.DataOffset == 124 {
			tune.Header.DataOffset = 0
		}
		if tune.Header.Songs == 1 {
			tune.Header.Songs = 0
		}
		if tune.Header.StartSong == 1 {
			tune.Header.StartSong = 0
		}
		if tune.Header.Name == "<?>" {
			tune.Header.Name = ""
		}
		if tune.Header.Author == "<?>" {
			tune.Header.Author = ""
		}
		if tune.Header.Released == "<?>" {
			tune.Header.Released = ""
		}
		Tunes[i] = tune
	}
}

// Set empty tune/header fields to default values after unmarshalling
func addDefaults() {
	for i, tune := range Tunes {
		tune.Index = i
		if tune.YearMax == 0 {
			tune.YearMax = tune.YearMin
		}
		if tune.Header.MagicID == "" {
			tune.Header.MagicID = "PSID"
		}
		if tune.Header.Version == 0 {
			tune.Header.Version = 2
		}
		if tune.Header.DataOffset == 0 {
			tune.Header.DataOffset = 124
		}
		if tune.Header.Songs == 0 {
			tune.Header.Songs = 1
		}
		if tune.Header.StartSong == 0 {
			tune.Header.StartSong = 1
		}
		Tunes[i] = tune
	}
}

func readReleases() {
	csdb.ReadReleases()
	for _, release := range csdb.Releases {
		sids := release.SIDs
		release.SIDs = nil
		for _, path := range sids {
			tuneIndex := TuneIndexByPath(path)
			if tuneIndex < 0 {
				log.Printf("Unknown path: %s", path)
				continue
			}
			Tunes[tuneIndex].Releases = append(Tunes[tuneIndex].Releases, release)
		}
	}
}

func tunesIndexPath() string {
	return filepath.Join(config.Config.AppBase, TunesIndexFile)
}

func detectRelease() {
	content, err := ioutil.ReadFile(hvscPathTo("DOCUMENTS/hv_sids.txt"))
	if err != nil {
		log.Fatalf("Unable to detect HVSC version: %s", err)
	}

	lr := regexp.MustCompile("[0-9]+")
	Release, err = strconv.Atoi(lr.FindString(string(content)))
	if err != nil {
		log.Fatalf("Unable to detect HVSC version from '%s'.", content)
	}
}
