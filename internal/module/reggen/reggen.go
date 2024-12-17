// Package reggen generates text based on regex definitions
// based on the reggen library by Lucas Jones
// https://github.com/lucasjones/reggen/blob/master/reggen.go
package reggen

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"regexp/syntax"
	"time"
)

const runeRangeEnd = 0x10ffff
const printableChars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~ \t\n\r"

var printableCharsNoNL = printableChars[:len(printableChars)-2]

type state struct {
	limit int
}

type Generator struct {
	re    *syntax.Regexp
	rand  *rand.Rand
	debug bool
}

func (g *Generator) generate(s *state, re *syntax.Regexp) string {
	switch re.Op {
	case syntax.OpNoMatch, syntax.OpEmptyMatch:
		return g.handleNoMatchOrEmptyMatch(re)
	case syntax.OpLiteral:
		return g.handleLiteral(re)
	case syntax.OpCharClass:
		return g.handleCharClass(s, re)
	case syntax.OpAnyCharNotNL, syntax.OpAnyChar:
		return g.handleAnyChar(re)
	case syntax.OpBeginLine, syntax.OpEndLine, syntax.OpBeginText, syntax.OpEndText, syntax.OpWordBoundary, syntax.OpNoWordBoundary:
		return ""
	case syntax.OpCapture:
		return g.handleCapture(s, re)
	case syntax.OpStar:
		return g.handleStar(s, re)
	case syntax.OpPlus:
		return g.handlePlus(s, re)
	case syntax.OpQuest:
		return g.handleQuest(s, re)
	case syntax.OpRepeat:
		return g.handleRepeat(s, re)
	case syntax.OpConcat:
		return g.handleConcat(s, re)
	case syntax.OpAlternate:
		return g.handleAlternate(s, re)
	default:
		fmt.Fprintln(os.Stderr, "[reg-gen] Unhandled op: ", re.Op)
		return ""
	}
}

func (*Generator) handleNoMatchOrEmptyMatch(_ *syntax.Regexp) string {
	return ""
}

func (*Generator) handleLiteral(re *syntax.Regexp) string {
	res := ""
	for _, r := range re.Rune {
		res += string(r)
	}
	return res
}

func (g *Generator) handleCharClass(_ *state, re *syntax.Regexp) string {
	sum := 0
	for i := 0; i < len(re.Rune); i += 2 {
		if g.debug {
			fmt.Printf("Range: %#U-%#U\n", re.Rune[i], re.Rune[i+1])
		}
		sum += int(re.Rune[i+1]-re.Rune[i]) + 1
		if re.Rune[i+1] == runeRangeEnd {
			sum = -1
			break
		}
	}
	if sum == -1 {
		return g.handleInverseCharClass(re)
	}
	if g.debug {
		fmt.Println("Char range: ", sum)
	}
	r := g.rand.Intn(sum)
	var ru rune
	sum = 0
	for i := 0; i < len(re.Rune); i += 2 {
		gap := int(re.Rune[i+1]-re.Rune[i]) + 1
		if sum+gap > r {
			ru = re.Rune[i] + rune(r-sum)
			break
		}
		sum += gap
	}
	if g.debug {
		fmt.Printf("Generated rune %c for range %v\n", ru, re)
	}
	return string(ru)
}

func (g *Generator) handleInverseCharClass(re *syntax.Regexp) string {
	possibleChars := []uint8{}
	for j := range printableChars {
		c := printableChars[j]
		for i := 0; i < len(re.Rune); i += 2 {
			if rune(c) >= re.Rune[i] && rune(c) <= re.Rune[i+1] {
				possibleChars = append(possibleChars, c)
				break
			}
		}
	}
	if len(possibleChars) > 0 {
		c := possibleChars[g.rand.Intn(len(possibleChars))]
		if g.debug {
			fmt.Printf("Generated rune %c for inverse range %v\n", c, re)
		}
		return string([]byte{c})
	}
	return ""
}

func (g *Generator) handleAnyChar(re *syntax.Regexp) string {
	chars := printableChars
	if re.Op == syntax.OpAnyCharNotNL {
		chars = printableCharsNoNL
	}
	c := chars[g.rand.Intn(len(chars))]
	return string([]byte{c})
}

func (g *Generator) handleCapture(s *state, re *syntax.Regexp) string {
	if g.debug {
		fmt.Println("OpCapture", re.Sub, len(re.Sub))
	}
	return g.generate(s, re.Sub0[0])
}

func (g *Generator) handleStar(s *state, re *syntax.Regexp) string {
	res := ""
	count := g.rand.Intn(s.limit + 1)
	for range make([]struct{}, count) {
		for _, r := range re.Sub {
			res += g.generate(s, r)
		}
	}
	return res
}

func (g *Generator) handlePlus(s *state, re *syntax.Regexp) string {
	res := ""
	count := g.rand.Intn(s.limit) + 1
	for range make([]struct{}, count) {
		for _, r := range re.Sub {
			res += g.generate(s, r)
		}
	}
	return res
}

func (g *Generator) handleQuest(s *state, re *syntax.Regexp) string {
	res := ""
	count := g.rand.Intn(2)
	if g.debug {
		fmt.Println("Quest", count)
	}
	for range make([]struct{}, count) {
		for _, r := range re.Sub {
			res += g.generate(s, r)
		}
	}
	return res
}

func (g *Generator) handleRepeat(s *state, re *syntax.Regexp) string {
	if g.debug {
		fmt.Println("OpRepeat", re.Min, re.Max)
	}
	res := ""
	count := 0
	re.Max = int(math.Min(float64(re.Max), float64(s.limit)))
	if re.Max > re.Min {
		count = g.rand.Intn(re.Max - re.Min + 1)
	}
	if g.debug {
		fmt.Println(re.Max, count)
	}
	for i := 0; i < re.Min || i < (re.Min+count); i++ {
		for _, r := range re.Sub {
			res += g.generate(s, r)
		}
	}
	return res
}

func (g *Generator) handleConcat(s *state, re *syntax.Regexp) string {
	res := ""
	for _, r := range re.Sub {
		res += g.generate(s, r)
	}
	return res
}

func (g *Generator) handleAlternate(s *state, re *syntax.Regexp) string {
	if g.debug {
		fmt.Println("OpAlternative", re.Sub, len(re.Sub))
	}
	i := g.rand.Intn(len(re.Sub))
	return g.generate(s, re.Sub[i])
}

// limit is the maximum number of times star, range or plus should repeat
// i.e. [0-9]+ will generate at most 10 characters if this is set to 10
func (g *Generator) Generate(limit int) string {
	return g.generate(&state{limit: limit}, g.re)
}

// create a new generator
func NewGenerator(regex string) (*Generator, error) {
	re, err := syntax.Parse(regex, syntax.Perl)
	if err != nil {
		return nil, err
	}
	return &Generator{
		re:   re,
		rand: rand.New(rand.NewSource(time.Now().UnixNano())), //nolint:gosec  // seed is not used for security
	}, nil
}

func (g *Generator) SetSeed(seed int64) {
	g.rand = rand.New(rand.NewSource(seed)) //nolint:gosec  // seed is not used for security
}

func Generate(regex string, limit int) (string, error) {
	g, err := NewGenerator(regex)
	if err != nil {
		return "", err
	}
	return g.Generate(limit), nil
}
