package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	_ "unsafe" // for go:linkname
)

var (
	startAddress = 0
	pc           = 0

	memory          = []uint8{}
	stack           = []int32{}
	rstack          = []int32{}
	jumps           = map[int]int{}
	runtimeDispatch = map[byte]func(){}

	program []byte
)

func compile() {
	for pc < len(program) {
		token := getToken()

		switch token {
		case '(':
			eatComment()
		case 'v':
			dataspaceLabel()
		case ':':
			defineFunction()
		case 'b':
			literalByteCompile()
		case '#':
			literalWord()
		case '*':
			allocateSpace()
		case '^':
			setStartAddress()
		case '[', '{':
			startConditional()
		case ']':
			endConditional()
		case '}':
			endLoop()
		case ' ', '\n':
			eatByte()
		case '\'':
			skipLiteralByte()
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			readNumber()
		}

		advancePastWhitespace()
	}
}

func run() {
	pc = startAddress
	for {
		token := getToken()
		// fmt.Printf("token=%q pc=%d stack=%d rstack=%d\n", token, pc, stack, rstack)

		switch token {
		case '(':
			jump()
		case 'W':
			writeOut()
		case 'G':
			readByte()
		case 'Q':
			quit()
		case '-':
			subtract()
		case '<':
			lessThan()
		case '@':
			fetch()
		case '!':
			store()
		case 's':
			storeByte()
		case ';':
			returnFromFunction()
		case '[':
			conditional()
		case ']', '{', ' ', '\n':
			nop()
		case '}':
			loop()
		case '\'':
			literalByteRun()
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			pushLiteral()
		default:
			runtimeDispatch[token]()
		}
	}
}

func literalByteRun() {
	eatByte()
	stack = append([]int32{int32(eatByte())}, stack...)
}

func loop() {
	var b int32
	b, stack = stack[0], stack[1:]

	if b == 0 {
		return
	}

	jump()
}

func nop() {

}

func conditional() {
	var b int32
	b, stack = stack[0], stack[1:]

	if b != 0 {
		return
	}

	jump()
}

func returnFromFunction() {
	pc, rstack = int(rstack[0]), rstack[1:]
}

func storeByte() {
	var addr, m int32
	addr, stack = stack[0], stack[1:]
	extend_memory(addr)
	m, stack = stack[0], stack[1:]

	memory[addr] = uint8(m)
}

func extend_memory(addr int32) {
	diff := addr + 1 - int32(len(memory))
	if diff > 0 {
		memory = append(memory, make([]uint8, diff)...)
	}
}

func store() {
	var addr, i int32
	addr, stack = stack[0], stack[1:]
	extend_memory(addr + 3)

	i, stack = stack[0], stack[1:]
	memory[addr] = uint8(i)
	memory[addr+1] = uint8(i >> 8)
	memory[addr+2] = uint8(i >> 16)
	memory[addr+3] = uint8(i >> 24)
}

func fetch() {
	var addr, n int32
	addr, stack = stack[0], stack[1:]
	n = int32(memory[addr]) | int32(memory[addr+1])<<8 | int32(memory[addr+2])<<16 | int32(memory[addr+3])<<24
	stack = append([]int32{n}, stack...)
}

func lessThan() {
	var a, b int32
	b, a, stack = int32(stack[0]), int32(stack[1]), stack[2:]
	if a < b {
		stack = append([]int32{1}, stack...)
	} else {
		stack = append([]int32{0}, stack...)
	}
}

func subtract() {
	var x, y int32
	x, y, stack = stack[0], stack[1], stack[2:]
	stack = append([]int32{y - x}, stack...)
}

func pushLiteral() {
	stack = append([]int32{int32(readNumber())}, stack...)
}

func quit() {
	// fmt.Printf("stack=%v\n", stack)
	// fmt.Printf("memory=%v\n", memory)
	os.Exit(0)
}

func readByte() {
	b := make([]byte, 1)
	_, err := os.Stdin.Read(b)
	if err != nil {
		stack = append([]int32{-1}, stack...)
	} else {
		stack = append([]int32{int32(b[0])}, stack...)
	}
}

func writeOut() {
	var count, address int32
	count, stack = stack[0], stack[1:]
	address, stack = stack[0], stack[1:]

	buf := strings.Builder{}
	for i := address; i < address+count; i++ {
		buf.WriteByte(memory[i])
	}

	fmt.Print(buf.String())
}

func jump() {
	pc = jumps[pc]
}

func skipLiteralByte() {
	eatByte() // '
	eatByte() // char
}

func endLoop() {
	var n int32
	n, stack = stack[0], stack[1:]
	jumps[pc] = int(n)
}

func startConditional() {
	stack = append([]int32{int32(pc)}, stack...)
}

func endConditional() {
	var n int32
	n, stack = stack[0], stack[1:]
	jumps[int(n)] = pc
}

func setStartAddress() {
	startAddress = pc
}

func allocateSpace() {
	advancePastWhitespace()
	n := readNumber()
	memory = append(memory, make([]uint8, n)...)
}

func literalByteCompile() {
	advancePastWhitespace()
	memory = append(memory, uint8(readNumber()))
}

func literalWord() {
	advancePastWhitespace()
	i := readNumber()
	b := []uint8{uint8(i), uint8(i >> 8), uint8(i >> 16), uint8(i >> 24)}
	memory = append(memory, b...)
}

func readNumber() uint32 {
	buf := strings.Builder{}
	for c := eatByte(); isDigit(c); c = eatByte() {
		buf.WriteByte(c)
	}

	i, _ := strconv.Atoi(buf.String())
	return uint32(i)
}

func pushDataspaceLabel(n int32) func() {
	return func() {
		stack = append([]int32{n}, stack...)
	}
}

func dataspaceLabel() {
	name := getToken()
	define(name, pushDataspaceLabel(int32(len(memory))))
}

func define(name byte, action func()) {
	runtimeDispatch[name] = action
}

func currentByte() byte {
	return program[pc]
}

func eatByte() byte {
	n := program[pc]
	pc++
	return n
}

func callFunction(n int) func() {
	return func() {
		rstack = append([]int32{int32(pc)}, rstack...)
		pc = n
	}
}

func defineFunction() {
	define(getToken(), callFunction(pc))
}

func eatComment() {
	commentStart := pc
	for eatByte() != ')' {
	}
	jumps[commentStart] = pc
}

func isWhitespace(c byte) bool {
	return c == ' ' || c == '\n' || c == '\t'
}

func isDigit(c byte) bool {
	return c-'0' < 10
}

func advancePastWhitespace() {
	for pc < len(program) && isWhitespace(currentByte()) {
		eatByte()
	}
}

func advanceToWhitespace() {
	for pc < len(program) && !isWhitespace(currentByte()) {
		eatByte()
	}
}

func getToken() byte {
	advancePastWhitespace()
	rv := currentByte()
	if !isDigit(rv) && rv != '\'' {
		advanceToWhitespace()
	}

	return rv
}

//go:linkname runtimeAbort runtime.abort
func runtimeAbort()

func main() {
	var err error
	if len(os.Args) != 2 {
		fmt.Println("wrong number of arguments")
		os.Exit(-1)
	}

	program, err = ioutil.ReadFile(os.Args[1])
	if err != nil {
		fmt.Printf("could not read file: %s", err)
		os.Exit(-1)
	}

	compile()

	run()

	runtimeAbort()
}
