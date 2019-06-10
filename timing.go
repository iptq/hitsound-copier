package main

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

var (
	// allow objects to be up to 2 milliseconds off
	ESTIMATE_THRESHOLD = 3.0

	// list of snappings that the editor uses
	SNAPPINGS = []int{1, 2, 3, 4, 6, 8, 12, 16}
)

type Timestamp interface {
	Milliseconds() int
}

type TimestampAbsolute int

func (t TimestampAbsolute) Milliseconds() int {
	return int(t)
}

type snapping struct {
	num   int
	denom int
	delta float64
}

type snappings []snapping

func (s snappings) Len() int {
	return len(s)
}

func (s snappings) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s snappings) Less(i, j int) bool {
	return s[i].delta < s[j].delta
}

// IntoRelative attempts to convert an absolute timestamp into a relative one
func (t TimestampAbsolute) IntoRelative(to Timestamp, bpm float64, meter int) (tp TimestampRelative, err error) {
	// return nil, fmt.Errorf("to = %+v", to)

	msPerBeat := 60000.0 / bpm
	msPerMeasure := msPerBeat * float64(meter)

	base := to.Milliseconds()
	cur := t.Milliseconds()

	measures := int(float64(cur-base) / msPerMeasure)
	measureStart := float64(base) + float64(measures)*msPerMeasure
	offset := float64(cur) - measureStart

	snapTimes := make([]snapping, len(SNAPPINGS)*16)
	for _, denom := range SNAPPINGS {
		for i := 0; i < denom; i++ {
			var snapAt float64

			snapAt = msPerMeasure * float64(i) / float64(denom)
			snapTimes = append(snapTimes, snapping{
				num:   i,
				denom: denom,
				delta: math.Abs(offset - snapAt),
			})

			snapAt = msPerMeasure * float64(i+denom) / float64(denom)
			snapTimes = append(snapTimes, snapping{
				num:   i + denom,
				denom: denom,
				delta: math.Abs(offset - snapAt),
			})
		}
	}
	sort.Sort(snappings(snapTimes))

	first := snapTimes[0]
	if first.delta > ESTIMATE_THRESHOLD {
		err = fmt.Errorf("Could not find accurate snapping.")
		return
	}

	tp = TimestampRelative{
		previous: to,
		bpm:      bpm,
		meter:    meter,
		measures: measures,
		num:      first.num,
		denom:    first.denom,
	}
	return
}

type TimestampRelative struct {
	previous Timestamp
	bpm      float64
	meter    int

	measures int
	num      int
	denom    int
}

func (t TimestampRelative) Milliseconds() int {
	// fmt.Println("previous:", t.previous, t.previous.Milliseconds())
	base := t.previous.Milliseconds()
	msPerBeat := 60000.0 / t.bpm
	msPerMeasure := msPerBeat * float64(t.meter)

	measureOffset := msPerMeasure * float64(t.measures)
	remainingOffset := msPerMeasure * float64(t.num) / float64(t.denom)
	return int(float64(base) + measureOffset + remainingOffset)
}

type TimingPoint interface {
	// Get the timestamp
	GetTimestamp() Timestamp

	// Get the BPM of the nearest uninherited timing section to which this belongs
	GetBPM() float64

	// Get the meter of the nearest uninherited timing section to which this belongs
	GetMeter() int

	GetSampleSetID() int
	GetCustomSampleIndex() int
	GetSampleVolume() int
	GetKiaiTimeActive() bool
}

type UninheritedTimingPoint struct {
	Time  Timestamp
	BPM   float64
	Meter int

	SampleSetID       int
	CustomSampleIndex int
	SampleVolume      int
	KiaiTimeActive    bool
}

func (tp *UninheritedTimingPoint) GetTimestamp() Timestamp {
	return tp.Time
}

func (tp *UninheritedTimingPoint) GetBPM() float64 {
	return tp.BPM
}

func (tp *UninheritedTimingPoint) GetMeter() int {
	return tp.Meter
}

func (tp *UninheritedTimingPoint) GetSampleSetID() int {
	return tp.SampleSetID
}

func (tp *UninheritedTimingPoint) GetCustomSampleIndex() int {
	return tp.CustomSampleIndex
}

func (tp *UninheritedTimingPoint) GetSampleVolume() int {
	return tp.SampleVolume
}

func (tp *UninheritedTimingPoint) GetKiaiTimeActive() bool {
	return tp.KiaiTimeActive
}

type InheritedTimingPoint struct {
	Parent       TimingPoint
	Time         Timestamp
	SvMultiplier float64

	SampleSetID       int
	CustomSampleIndex int
	SampleVolume      int
	KiaiTimeActive    bool
}

func (tp *InheritedTimingPoint) GetTimestamp() Timestamp {
	return tp.Time
}

func (tp *InheritedTimingPoint) GetBPM() float64 {
	return tp.Parent.GetBPM()
}

func (tp *InheritedTimingPoint) GetMeter() int {
	return tp.Parent.GetMeter()
}

func (tp *InheritedTimingPoint) GetSampleSetID() int {
	return tp.SampleSetID
}

func (tp *InheritedTimingPoint) GetCustomSampleIndex() int {
	return tp.CustomSampleIndex
}

func (tp *InheritedTimingPoint) GetSampleVolume() int {
	return tp.SampleVolume
}

func (tp *InheritedTimingPoint) GetKiaiTimeActive() bool {
	return tp.KiaiTimeActive
}

func ParseTimingPoint(parent TimingPoint, line string) (tp TimingPoint, err error) {
	var beatLength float64
	var offset, meter, sampleSetID, customSampleIndex, sampleVolume int
	var kiai bool

	parts := strings.Split(line, ",")
	if offset, err = strconv.Atoi(parts[0]); err != nil {
		return
	}
	if beatLength, err = strconv.ParseFloat(parts[1], 64); err != nil {
		beatLength = 0
	}
	if meter, err = strconv.Atoi(parts[2]); err != nil {
		return
	}
	if len(parts) > 3 {
		if sampleSetID, err = strconv.Atoi(parts[3]); err != nil {
			return
		}
	}
	if len(parts) > 4 {
		if customSampleIndex, err = strconv.Atoi(parts[4]); err != nil {
			return
		}
	}
	if len(parts) > 5 {
		if sampleVolume, err = strconv.Atoi(parts[5]); err != nil {
			return
		}
	} else {
		sampleVolume = 100
	}
	if len(parts) > 7 {
		var x int
		x, err = strconv.Atoi(parts[7])
		if err != nil {
			return
		}
		kiai = (x == 1)
	}

	if beatLength == 0 {
		return nil, fmt.Errorf("beat length is equal to 0")
	} else if beatLength > 0 {
		tp = &UninheritedTimingPoint{
			BPM:   math.Trunc(60000.0/beatLength + 0.5),
			Meter: meter,
			Time:  TimestampAbsolute(offset),

			SampleSetID:       sampleSetID,
			CustomSampleIndex: customSampleIndex,
			SampleVolume:      sampleVolume,
			KiaiTimeActive:    kiai,
		}
	} else {
		var time Timestamp
		time = TimestampAbsolute(offset)
		time, err = time.(TimestampAbsolute).IntoRelative(parent.GetTimestamp(), parent.GetBPM(), parent.GetMeter())
		if err != nil {
			return
		}

		tp = &InheritedTimingPoint{
			Parent:       parent,
			Time:         time,
			SvMultiplier: math.Abs(100.0 / beatLength),

			SampleSetID:       sampleSetID,
			CustomSampleIndex: customSampleIndex,
			SampleVolume:      sampleVolume,
			KiaiTimeActive:    kiai,
		}
	}
	return
}
