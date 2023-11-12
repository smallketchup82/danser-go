package settings

import (
	"fmt"
	"log"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"
)

var libxProfiles = []string{
	"baseline",
	"main",
	"high",
}

var libxPresets = []string{
	"ultrafast",
	"superfast",
	"veryfast",
	"faster",
	"fast",
	"medium",
	"slow",
	"slower",
	"veryslow",
	"placebo",
}

type x264Settings struct {
	RateControl       string `combo:"vbr|VBR,cbr|CBR,crf|Constant Rate Factor (CRF)"`
	TargetSize        string `showif:"RateControl=cbr" tooltip:"Target size in megabytes. Bitrate option below is ignored if this is set (not empty)"`
	Bitrate           string `showif:"RateControl=vbr,cbr"`
	CRF               int    `string:"true" min:"0" max:"51" showif:"RateControl=crf"`
	Profile           string `combo:"baseline,main,high"`
	Preset            string `combo:"ultrafast,superfast,veryfast,faster,fast,medium,slow,slower,veryslow,placebo"`
	AdditionalOptions string
}

func (s *x264Settings) GenerateFFmpegArgs(length float64) (ret []string, err error) {

	if s.TargetSize != "" {
		length = length / 1000
		log.Println("length: ", length)
		var targetsize, _ = strconv.Atoi(s.TargetSize)
		// Convert targetsize from MB to MiB. Then convert from MiB to kBit. Then divide by length to get the bitrate
		var newbitrate = ((float64(targetsize) / 1.049) * 8388.608) / length

		// Subtract the audio bitrate from the video bitrate to get the video bitrate
		// Wait until the audio bitrate is set before doing this
		for AudioBitrate == "" || AudioBitrate == "0" {
			log.Println("Waiting for audio bitrate to be set...")
			time.Sleep(1 * time.Second)
		}

		// Convert AudioBitrate to int (remove the "k" at the end)
		var audiobitrate, _ = strconv.Atoi(AudioBitrate[:len(AudioBitrate)-1])
		log.Println("audiobitrate: ", audiobitrate)

		newbitrate = newbitrate - float64(audiobitrate)

		// Convert newbitrate to a string and set it to s.Bitrate
		var newerbitrate = strconv.FormatFloat(newbitrate, 'f', 3, 64)
		s.Bitrate = newerbitrate + "k"
		log.Println("Bitrate must be set to:", s.Bitrate, "to achieve target size of", s.TargetSize, "MB")
	}

	ret, err = libxCommon(s.RateControl, s.Bitrate, s.CRF)
	if err != nil {
		return nil, err
	}

	if !slices.Contains(libxProfiles, s.Profile) {
		return nil, fmt.Errorf("invalid profile: %s", s.Profile)
	}

	ret = append(ret, "-profile", s.Profile)

	ret2, err := libxCommon2(s.Preset, s.AdditionalOptions)
	if err != nil {
		return nil, err
	}

	return append(ret, ret2...), nil
}

type x265Settings struct {
	RateControl       string `combo:"vbr|VBR,cbr|CBR,crf|Constant Rate Factor (CRF)"`
	Bitrate           string `showif:"RateControl=vbr,cbr"`
	CRF               int    `string:"true" min:"0" max:"51" showif:"RateControl=crf"`
	Preset            string `combo:"ultrafast,superfast,veryfast,faster,fast,medium,slow,slower,veryslow,placebo"`
	AdditionalOptions string
}

func (s *x265Settings) GenerateFFmpegArgs(length float64) (ret []string, err error) {
	ret, err = libxCommon(s.RateControl, s.Bitrate, s.CRF)
	if err != nil {
		return nil, err
	}

	ret2, err := libxCommon2(s.Preset, s.AdditionalOptions)
	if err != nil {
		return nil, err
	}

	return append(ret, ret2...), nil
}

func libxCommon(rateControl, bitrate string, crf int) (ret []string, err error) {
	switch strings.ToLower(rateControl) {
	case "vbr":
		ret = append(ret, "-b:v", bitrate)
	case "cbr":
		ret = append(ret, "-b:v", bitrate, "-minrate", bitrate, "-maxrate", bitrate, "-bufsize", bitrate)
	case "crf":
		if crf < 0 {
			return nil, fmt.Errorf("CRF parameter out of range [0-%d]", math.MaxInt)
		}

		ret = append(ret, "-crf", strconv.Itoa(crf))
	default:
		return nil, fmt.Errorf("invalid rate control value: %s", rateControl)
	}

	return
}

func libxCommon2(preset string, additional string) (ret []string, err error) {
	if !slices.Contains(libxPresets, preset) {
		return nil, fmt.Errorf("invalid preset: %s", preset)
	}

	ret = append(ret, "-preset", preset)

	ret = parseCustomOptions(ret, additional)

	return
}
