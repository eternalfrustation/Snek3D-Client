package main

import (
	"encoding/binary"
	"fmt"
	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/go-gl/mathgl/mgl32"
	"image"
	"image/png"
	"io/ioutil"
	"os"
	"runtime"
	"time"
	"unsafe"
)

const (
	W         = 500
	H         = 500
	fps       = time.Second / 60
	pi        = 3.1415926535897932384626433832795028841971693993751058209749445923078164062862089986280348253421170679
	viewRange = 1000
	// The first point of the array of the Vectors in the ray struct
	// is used as initial point for subsequent rays, for eg.
	// Consider for following array : [{0, 1}, {13, 23}, {23, 24}, {1, 23}]
	// There will be a total of 3 rays constructed from the array and
	// they will be intersecting [{0, 1}, {13, 23}], [{0, 1}, {23, 24}]
	// and [{0, 1}, {1, 23}] respectively
	RAY_TYPE_CENTERED = 0x0
	// The initial point in the array of vectors in the ray struct
	// is every other vector, or the index has the index 2n where
	// n is the number of ray being considered, for eg.

	// Consider for following array : [{0, 1}, {13, 23}, {23, 24}, {1, 23}]
	// There will be a total of 2 rays constructed from the array and
	// they will be intersecting [{0, 1}, {13, 23}] and [{23, 24}, {1, 23}]
	// respectively
	RAY_TYPE_STRIP = 0x1
	pointByteSize  = int32(13 * 4)
)

var (
	viewMat        mgl32.Mat4
	projMat        mgl32.Mat4
	defaultViewMat mgl32.Mat4
	AddState       byte
	program        uint32
	MouseX         float64
	MouseY         float64
	CurrPoint      mgl32.Vec2
	Btns           []*Button
	BtnState       = byte('C')
	eyePos         mgl32.Vec3
	LookAt         mgl32.Vec3
	MouseRay       *Ray
	framesDrawn    int
	Ident          = mgl32.Ident4()
	endianness     binary.ByteOrder
	Snake []*Shape
	Food *Shape
	inputFile *os.File
	outputFile *os.File
)

func main() {
	inputName := os.Args[1]
	outputName := os.Args[2]
	inputFile = os.Stdin
	outputFile = os.Stdout
	var err error
	if inputName != "-" {
		inputFile, err = os.Open(inputName)
		orDie(err)
	}
	if outputName != "-" {
		outputFile, err = os.Open(outputName)
		orDie(err)
	}
	var i int32 = 0x1
	bs := (*[4]byte)(unsafe.Pointer(&i))
	if bs[0] == 0 {
		endianness = binary.BigEndian
	} else {
		endianness = binary.LittleEndian
	}
	runtime.LockOSThread()
	orDie(glfw.Init())
	// Close glfw when main exits
	defer glfw.Terminate()

	// Window Properties
	glfw.WindowHint(glfw.Resizable, glfw.True)
	glfw.WindowHint(glfw.ContextVersionMajor, 4)
	glfw.WindowHint(glfw.ContextVersionMinor, 1)
	glfw.WindowHint(glfw.OpenGLProfile, glfw.OpenGLCoreProfile)
	glfw.WindowHint(glfw.OpenGLForwardCompatible, glfw.True)
	// Create the window with the above hints
	window, err := glfw.CreateWindow(W, H, "Bvg Editor", nil, nil)
	orDie(err)
	// Load the icon file
	icoFile, err := os.Open("ico.png")
	orDie(err)
	// decode the file to an image.Image
	ico, err := png.Decode(icoFile)
	orDie(err)
	window.SetIcon([]image.Image{ico})
	window.MakeContextCurrent()
	// OpenGL Initialization
	// Check for the version
	//version := gl.GoStr(gl.GetString(gl.VERSION))
	//	fmt.Println("OpenGL Version", version)
	// Read the vertex and fragment shader files
	vertexShader, err := ioutil.ReadFile("vertex.vert")
	orDie(err)
	vertexShader = append(vertexShader, []byte("\x00")...)
	fragmentShader, err := ioutil.ReadFile("frag.frag")
	orDie(err)
	fragmentShader = append(fragmentShader, []byte("\x00")...)

	orDie(gl.Init())

	// Set the function for handling errors
	gl.DebugMessageCallback(func(source, gltype, id, severity uint32, length int32, message string, userParam unsafe.Pointer) {
		panic(fmt.Sprintf("%d, %d, %d, %d, %d, %s \n", source, gltype, severity, id, length, message))

	}, nil)
	// Create an OpenGL "Program" and link it for current drawing
	prog, err := newProg(string(vertexShader), string(fragmentShader))
	orDie(err)
	// Check for the version
	version := gl.GoStr(gl.GetString(gl.VERSION))
	fmt.Println("OpenGL Version", version)
	// Main draw loop

	// Set the refresh function for the window
	// Use this program
	gl.UseProgram(prog)
	// Calculate the projection matrix
	projMat = mgl32.Ident4()
	// set the value of Projection matrix
	UpdateUniformMat4fv("projection", program, &projMat[0])
	// Set the value of view matrix
	UpdateView(
		mgl32.Vec3{0, 0, -1},
		mgl32.Vec3{0, 0, 1},
	)
	program = prog
	// GLFW Initialization
	CurrPoint = mgl32.Vec2{0, 0}
	eyePos = mgl32.Vec3{0, 0, 1}

	for !window.ShouldClose() {
		time.Sleep(fps)
		// Clear everything that was drawn previously
		gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)
		// Actually draw something
		//		b.Draw()
		framesDrawn++
		for _, v := range Snake {
			v.GenVao()
		}
		Food.Draw()
		//		fnt.GlyphMap['e'].Draw()
		// display everything that was drawn
		window.SwapBuffers()
		// check for any events
		glfw.PollEvents()
	}
}

func NextFrame() (SnekPos []mgl32.Vec3, foodPos mgl32.Vec3) {
	lenPointsBytes := make([]byte, 2)
	inputFile.Read(lenPointsBytes)
	lenPoints := binary.BigEndian.Uint16(lenPointsBytes)
	lenBitsBytes := make([]byte, 1)
	inputFile.Read(lenBitsBytes)
	lenBits := lenBitsBytes[0]
	coordBytes := make([]byte, lenBits>>3)
	var bytesToFloats func(inputBytes []byte) float32
	switch lenBits {
	case 8:
		bytesToFloats = func(inputBytes []byte) float32 {
			return float32(inputBytes[0]) / float32(1<<8-1)
		}
	case 16:
		bytesToFloats = func(inputByte []byte) float32 {
			U16int := binary.BigEndian.Uint16(inputByte)
			return float32(U16int) / float32(1>>16-1)
		}
	case 32:
		bytesToFloats = func(inputByte []byte) float32 {
			U32int := binary.BigEndian.Uint32(inputByte)
			return float32(U32int) / float32(1<<32-1)
		}
	case 64:
		bytesToFloats = func(inputByte []byte) float32 {
			U64int := binary.BigEndian.Uint64(inputByte)
			return float32(U64int) / float32(1<<64-1)
		}
	}
	inputFile.Read(coordBytes)
	x := bytesToFloats(coordBytes)
	inputFile.Read(coordBytes)
	y := bytesToFloats(coordBytes)
	inputFile.Read(coordBytes)
	z := bytesToFloats(coordBytes)
	foodPos = mgl32.Vec3{x, y, z}
	for i := uint64(3 + uint64(lenBits>>3)*3); i < uint64(lenPoints*uint16(lenBits>>3)); i += uint64(lenBits>>3) * 3 {
		inputFile.Read(coordBytes)
		x := bytesToFloats(coordBytes)
		inputFile.Read(coordBytes)
		y := bytesToFloats(coordBytes)
		inputFile.Read(coordBytes)
		z := bytesToFloats(coordBytes)
		SnekPos = append(SnekPos, mgl32.Vec3{x, y, z})
	}
	return SnekPos, foodPos
}
