package audio

import (
	"bytes"
	"encoding/binary"
	"log"
	"math"
	"sync"
	"time"

	"github.com/ebitengine/oto/v3"
)

const (
	SampleRate      = 44100
	ChannelCount    = 2
	BitDepthInBytes = 2
	BaseFreq        = 293.66
	MaxLineLength   = 120.0
)

// Synth wraps the Oto context for generating audio
type Synth struct {
	ctx           *oto.Context
	currentPlayer *oto.Player
	mu            sync.Mutex
}

// NewSynth initializes the stereo PCM audio context
func NewSynth() (*Synth, error) {
	op := &oto.NewContextOptions{
		SampleRate:   SampleRate,
		ChannelCount: ChannelCount,
		Format:       oto.FormatSignedInt16LE,
	}

	ctx, ready, err := oto.NewContext(op)
	if err != nil {
		return nil, err
	}
	// Wait for the audio context to be ready
	<-ready

	return &Synth{
		ctx: ctx,
	}, nil
}

// PlaySonar plays a tone corresponding to the AST depth and column position
func (s *Synth) PlaySonar(depth int, col int) {
	dimScale := []float64{0, 2, 3, 5, 6, 8, 9, 11}
	octave := depth / len(dimScale)
	noteIdx := depth % len(dimScale)
	
	semitones := float64(octave*12) + dimScale[noteIdx]
	freq := BaseFreq * math.Pow(2, semitones/12.0)
	
	log.Printf("Synthesizing D-Diminished Note: Depth %d -> %f Hz (+%v semitones)", depth, freq, semitones)

	// Spatial Panning: value between -1.0 (left) and 1.0 (right)
	pan := (float64(col)/MaxLineLength)*2.0 - 1.0
	if pan > 1.0 {
		pan = 1.0
	} else if pan < -1.0 {
		pan = -1.0
	}

	// Generate 500ms of PCM data
	duration := 500 * time.Millisecond
	numSamples := int(float64(SampleRate) * duration.Seconds())
	buf := new(bytes.Buffer)

	// ADSR Envelope Configuration for a pleasant, softer sound
	attackSamples := int(float64(SampleRate) * 0.08) // 80ms Attack (softer fade in)
	decaySamples := int(float64(SampleRate) * 0.2)   // 200ms Decay
	sustainLevel := 0.4                              // Quieter sustain
	releaseSamples := int(float64(SampleRate) * 0.2) // 200ms Release (smooth fade out)
	sustainSamples := numSamples - attackSamples - decaySamples - releaseSamples

	if sustainSamples < 0 {
		sustainSamples = 0
	}

	for i := 0; i < numSamples; i++ {
		t := float64(i) / float64(SampleRate)
		
		// Additive synthesis for a richer, more pleasant "electric piano / bell" tone
		sample := math.Sin(2.0*math.Pi*freq*t) +
			0.4*math.Sin(2.0*math.Pi*(freq*2)*t) +
			0.2*math.Sin(2.0*math.Pi*(freq*3)*t) +
			0.1*math.Sin(2.0*math.Pi*(freq*4)*t)
		sample /= 1.7 // Normalize to prevent clipping

		// Envelope Generation (ADSR)
		var env float64
		if i < attackSamples {
			env = float64(i) / float64(attackSamples)
		} else if i < attackSamples+decaySamples {
			env = 1.0 - (1.0-sustainLevel)*float64(i-attackSamples)/float64(decaySamples)
		} else if i < attackSamples+decaySamples+sustainSamples {
			env = sustainLevel
		} else {
			env = sustainLevel * (1.0 - float64(i-(attackSamples+decaySamples+sustainSamples))/float64(releaseSamples))
		}
		if env < 0 {
			env = 0
		}

		sample *= env

		// Volume and Panning (Constant Power)
		volume := 0.5
		leftVolume := volume * math.Cos((pan+1.0)*math.Pi/4.0)
		rightVolume := volume * math.Sin((pan+1.0)*math.Pi/4.0)

		leftSample := int16(sample * leftVolume * math.MaxInt16)
		rightSample := int16(sample * rightVolume * math.MaxInt16)

		binary.Write(buf, binary.LittleEndian, leftSample)
		binary.Write(buf, binary.LittleEndian, rightSample)
	}

	player := s.ctx.NewPlayer(buf)

	// Prevent polyphony crackle by killing the old player
	s.mu.Lock()
	if s.currentPlayer != nil {
		s.currentPlayer.Pause()
	}
	s.currentPlayer = player
	s.mu.Unlock()

	player.Play()

	// Keep alive to prevent GC while playing
	go func(p *oto.Player) {
		for p.IsPlaying() {
			time.Sleep(20 * time.Millisecond)
		}
	}(player)
}
