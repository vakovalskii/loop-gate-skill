package main

import (
	"hash/maphash"
	"unicode"
	"unicode/utf8"
)

// Детектор цикла по ходу генерации (эталон из прода NeuralDeep, Go без
// зависимостей). Кормите его дельтами content и reasoning отдельно. N-словная последовательность встретилась не меньше
// MinRepeat раз И доля слов в повторах не ниже MinRatio. Взводится после
// ArmWords слов: до этого по калибровке циклов нет, а честные короткие ответы
// со списками и таблицами дают ложные повторы.
//
// N-грамма считается кольцевым полиномиальным хешем от хешей слов, поэтому
// новое слово стоит O(1), а не O(N).

type DetectorConfig struct {
	NGram     int
	ArmWords  int
	MinRepeat int
	MinRatio  float64
}

type Fire struct {
	Words  int     `json:"words"`
	Secs   float64 `json:"secs"`
	Repeat int     `json:"repeat"`
	Ratio  float64 `json:"ratio"`
}

type Channel struct {
	cfg    DetectorConfig
	seed   maphash.Seed
	pow    uint64   // B^N для выкидывания старого слова из окна
	window []uint64 // хеши последних N слов, кольцо
	head   int
	roll   uint64
	words  int
	tail   []byte // недописанное слово с конца прошлой дельты
	counts map[uint64]int32
	rep    int
	top    int
	Fired  *Fire
}

const rollBase = 1099511628211

func NewChannel(cfg DetectorConfig, seed maphash.Seed) *Channel {
	pow := uint64(1)
	for i := 0; i < cfg.NGram; i++ {
		pow *= rollBase
	}
	return &Channel{cfg: cfg, seed: seed, pow: pow, window: make([]uint64, cfg.NGram),
		counts: make(map[uint64]int32)}
}

func (c *Channel) Words() int { return c.words }
func (c *Channel) Top() int   { return c.top }

func (c *Channel) Ratio() float64 {
	grams := c.words - c.cfg.NGram + 1
	if grams <= 0 {
		return 0
	}
	return float64(c.rep) / float64(grams)
}

func (c *Channel) push(word []byte) {
	h := maphash.Bytes(c.seed, word) | 1
	old := c.window[c.head]
	c.window[c.head] = h
	c.head = (c.head + 1) % c.cfg.NGram
	c.roll = c.roll*rollBase + h - old*c.pow
	c.words++
	if c.words < c.cfg.NGram {
		return
	}
	n := c.counts[c.roll] + 1
	c.counts[c.roll] = n
	switch {
	case n == 2:
		c.rep += 2
	case n > 2:
		c.rep++
	}
	if int(n) > c.top {
		c.top = int(n)
	}
}

// Feed режет дельту на слова по пробельным символам; хвост без пробела ждёт
// продолжения в следующей дельте.
func (c *Channel) Feed(s string) {
	start := -1
	if len(c.tail) > 0 {
		start = 0
	}
	buf := append(c.tail, s...)
	c.tail = nil
	for i := 0; i < len(buf); {
		r, size := utf8.DecodeRune(buf[i:])
		if unicode.IsSpace(r) {
			if start >= 0 {
				c.push(buf[start:i])
				start = -1
			}
		} else if start < 0 {
			start = i
		}
		i += size
	}
	if start >= 0 {
		c.tail = append([]byte(nil), buf[start:]...)
	}
}

func (c *Channel) Flush() {
	if len(c.tail) > 0 {
		c.push(c.tail)
		c.tail = nil
	}
}

func (c *Channel) Check(elapsed float64) {
	if c.Fired != nil || c.words < c.cfg.ArmWords {
		return
	}
	if c.top >= c.cfg.MinRepeat && c.Ratio() >= c.cfg.MinRatio {
		c.Fired = &Fire{Words: c.words, Secs: round1(elapsed), Repeat: c.top, Ratio: round3(c.Ratio())}
	}
}

// Looping - боевой критерий (06.10.26): то же, что у сработки, но доля
// повтора не ниже cut и считается на каждом событии, а не один раз. По архиву
// 05-06.10 живые ответы в выборке ни разу не дошли до 0.35, циклы уходят к 0.8-0.99.
func (c *Channel) Looping(cut float64) bool {
	return c.words >= c.cfg.ArmWords && c.top >= c.cfg.MinRepeat && c.Ratio() >= cut
}

func round1(f float64) float64 { return float64(int64(f*10+0.5)) / 10 }
func round3(f float64) float64 { return float64(int64(f*1000+0.5)) / 1000 }
