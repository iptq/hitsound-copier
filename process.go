package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

var (
	SECTION_PATTERN = regexp.MustCompile(`^\[([[:alpha:]]+)\]$`)
)

type HitsoundData struct {
	TimingPoints []TimingPoint
	Hitsounds    map[string]Hitsound
}

type Volume struct{}

type Hitsound struct {
	Timestamp Timestamp
	Additions int
}

func computeSig(t Timestamp) string {
	if ts, ok := t.(TimestampAbsolute); ok {
		return fmt.Sprintf("a:%d", ts)
	} else if ts, ok := t.(TimestampRelative); ok {
		return fmt.Sprintf("r:(%s):%f:%d:%d:%d:%d", computeSig(ts.previous), ts.bpm, ts.meter, ts.measures, ts.num, ts.denom)
	}
	return "error"
}

func collectHitsounds(reader io.Reader) (hsdata HitsoundData, err error) {
	var buf []byte
	var section string

	bufreader := bufio.NewReader(reader)
	hitsounds := make(map[string]Hitsound)
	hitsoundsArr := make([]Hitsound, 0)
	timingPointLines := make([]string, 0)
	for nLine := -1; err == nil; buf, _, err = bufreader.ReadLine() {
		nLine += 1
		line := strings.Trim(string(buf), " \r\n")
		if len(line) == 0 {
			// empty line
			continue
		}

		// update current section
		if match := SECTION_PATTERN.FindStringSubmatch(line); match != nil {
			section = match[1]
			continue
		}

		switch strings.ToLower(section) {
		case "timingpoints":
			timingPointLines = append(timingPointLines, line)
		case "hitobjects":
			var offset, objType, additions int

			parts := strings.Split(line, ",")
			if offset, err = strconv.Atoi(parts[2]); err != nil {
				return
			}
			if objType, err = strconv.Atoi(parts[3]); err != nil {
				err = errors.Wrap(err, "invalid hit object type: "+parts[3])
				return
			}
			if additions, err = strconv.Atoi(parts[4]); err != nil {
				return
			}

			if (objType & 1) == 1 {
				// hit circle
				hs := Hitsound{
					Timestamp: TimestampAbsolute(offset),
					Additions: additions,
				}
				hitsoundsArr = append(hitsoundsArr, hs)
			} else if (objType & 2) == 2 {
				// slider
				hs := Hitsound{
					Timestamp: TimestampAbsolute(offset),
					Additions: additions,
				}
				hitsoundsArr = append(hitsoundsArr, hs)
			} else if (objType & 8) == 8 {
				// spinner
				var endTime int
				if offset, err = strconv.Atoi(parts[5]); err != nil {
					return
				}
				hs := Hitsound{
					Timestamp: TimestampAbsolute(endTime),
					Additions: additions,
				}
				hitsoundsArr = append(hitsoundsArr, hs)
			}
		}
	}

	// sort timing points by offset
	sort.Slice(timingPointLines, func(i, j int) bool {
		p1 := strings.Split(timingPointLines[i], ",")
		p2 := strings.Split(timingPointLines[i], ",")
		o1, _ := strconv.Atoi(p1[1])
		o2, _ := strconv.Atoi(p2[1])
		return o1 < o2
	})

	timingPoints := make([]TimingPoint, 1)
	uninheriteds := make([]TimingPoint, 1)
	if len(timingPointLines) == 0 {
		err = fmt.Errorf("no timing points?")
		return
	}

	// first one should always be uninherited
	var tp, prev TimingPoint
	prev, err = ParseTimingPoint(nil, timingPointLines[0])
	if err != nil {
		return
	}

	timingPoints = append(timingPoints, prev)
	uninheriteds = append(uninheriteds, prev)
	for i, line := range timingPointLines {
		if i == 0 {
			continue
		}

		tp, err = ParseTimingPoint(prev, line)
		if _, ok := tp.(*UninheritedTimingPoint); ok {
			uninheriteds = append(uninheriteds, tp)
			prev = tp
		}
		timingPoints = append(timingPoints, tp)
	}

	// sort hitsounds by offset
	sort.Slice(hitsoundsArr, func(i, j int) bool {
		a := hitsoundsArr[i].Timestamp.Milliseconds()
		b := hitsoundsArr[i].Timestamp.Milliseconds()
		return a < b
	})

	i := 0
	for j, hs := range hitsoundsArr {
		if i < len(uninheriteds)-1 && hs.Timestamp.Milliseconds() >= uninheriteds[i+1].GetTimestamp().Milliseconds() {
			i++
		}
		tp = uninheriteds[i]
		hitsoundsArr[j].Timestamp, err = hs.Timestamp.(TimestampAbsolute).IntoRelative(tp.GetTimestamp(), tp.GetBPM(), tp.GetMeter())
		if err != nil {
			return
		}

		sig := computeSig(hitsoundsArr[j].Timestamp)
		hitsounds[sig] = hitsoundsArr[j]
	}

	hsdata.TimingPoints = timingPoints
	hsdata.Hitsounds = hitsounds
	return
}

func applyHitsounds(hsdata HitsoundData, data *bytes.Buffer, writer io.Writer) (err error) {
	var buf []byte

	section := "version"
	sections := make(map[string][]string)

	bufreader := bufio.NewReader(data)
	for ; err == nil; buf, _, err = bufreader.ReadLine() {
		line := strings.Trim(string(buf), " \r\n")
		if len(line) == 0 {
			// empty line
			continue
		}

		// update current section
		if match := SECTION_PATTERN.FindStringSubmatch(line); match != nil {
			section = match[1]
			continue
		}

		switch strings.ToLower(section) {
		case "timingpoints":
		case "hitobjects":
		default:
			sections[section] = append(sections[section], line)
		}
	}

	order := []string{"version", "General", "Editor", "Metadata", "Difficulty", "Events", "TimingPoints", "Colours", "HitObjects"}
	for _, section := range order {
		lines, ok := sections[section]
		if !ok {
			continue
		}

		if section != "version" {
			writer.Write([]byte("\r\n[" + section + "]\r\n"))
		}
		for _, line := range lines {
			writer.Write([]byte(line + "\r\n"))
		}
	}

	return
}

func copyHitsounds(fromFile, toFile string, backup bool) (err error) {
	// collect hitsounds
	fmt.Println(fromFile)
	f, err := os.Open(fromFile)
	if err != nil {
		return
	}
	hsdata, err := collectHitsounds(f)
	for _, tp := range hsdata.Hitsounds {
		fmt.Println(tp)
	}
	if err := f.Close(); err != nil {
		log.Fatal(err)
	}

	// apply hitsounds
	toData, err := ioutil.ReadFile(toFile)
	if err != nil {
		return
	}
	if backup {
		err = ioutil.WriteFile(toFile+".bak", toData, 0644)
		if err != nil {
			return
		}
	}

	f, err = os.OpenFile(toFile, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return
	}
	buf := bytes.NewBuffer(toData)
	applyHitsounds(hsdata, buf, f)
	if err := f.Close(); err != nil {
		log.Fatal(err)
	}

	return nil
}
