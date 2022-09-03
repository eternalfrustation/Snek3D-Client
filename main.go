package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"image"
	"image/png"
	"io/ioutil"
	"net/url"
	"os"
	"runtime"
	"time"
	"unsafe"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/go-gl/mathgl/mgl32"
	"golang.org/x/net/websocket"
)

const (
	W         = 500
	H         = 500
	fps       = time.Second / 2
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
	viewMat                         mgl32.Mat4
	projMat                         mgl32.Mat4
	defaultViewMat                  mgl32.Mat4
	AddState                        byte
	program                         uint32
	MouseX                          float64
	MouseY                          float64
	CurrPoint                       mgl32.Vec2
	Btns                            []*Button
	BtnState                        = byte('C')
	eyePos                          mgl32.Vec3
	LookAt                          mgl32.Vec3
	MouseRay                        *Ray
	framesDrawn                     int
	Ident                           = mgl32.Ident4()
	endianness                      binary.ByteOrder
	coordBytes                      []byte
	bytesToU64                      func(inputBytes []byte) uint64
	lenPoints                       uint16
	maxWorldX, maxWorldY, maxWorldZ float32
	addr                            = flag.String("addr", "127.0.0.1:6969", "Snek Server address with port")
	score                           uint32
	gameEnded                       bool
	lenInt                          uint8
	keysPressed                     keyStack
)

func main() {
	flag.Parse()
	u := url.URL{Scheme: "ws", Host: *addr}
	c, err := websocket.Dial(u.String(), "", "http://"+u.Hostname())
	fmt.Println(u.String(), u.Hostname())
	orDie(err)
	defer c.Close()

	buf := [2]byte{}
	*(*uint16)(unsafe.Pointer(&buf[0])) = uint16(0xABCD)

	switch buf {
	case [2]byte{0xCD, 0xAB}:
		endianness = binary.LittleEndian
	case [2]byte{0xAB, 0xCD}:
		endianness = binary.BigEndian
	default:
		panic("Could not determine native endianness.")
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
	window, err := glfw.CreateWindow(W, H, "Snek3D-Frontend", nil, nil)
	orDie(err)
	window.Focus()
	window.Maximize()
	window.Restore()
	// Load the icon file
	icoFile, err := os.Open("ico.png")
	orDie(err)
	// decode the file to an image.Image
	ico, err := png.Decode(icoFile)
	orDie(err)
	window.SetIcon([]image.Image{ico})
	window.MakeContextCurrent()
	window.SetKeyCallback(StoreKeys)
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
	WhiteCube := NewShape(Ident, program, []*Point{
		PC(1, 1, 1, 1, 1, 1, 1),
		PC(-1, 1, 1, 1, 1, 1, 1),
		PC(-1, -1, 1, 1, 1, 1, 1),
		PC(-1, -1, -1, 1, 1, 1, 1),
		PC(1, -1, -1, 1, 1, 1, 1),
		PC(1, 1, -1, 1, 1, 1, 1),
		PC(1, -1, 1, 1, 1, 1, 1),
		PC(-1, 1, -1, 1, 1, 1, 1),
	}...)
	RedCube := NewShape(Ident, program, []*Point{
		PC(1, 1, 1, 1, 0, 0, 1),
		PC(-1, 1, 1, 1, 0, 0, 1),
		PC(-1, -1, 1, 1, 0, 0, 1),
		PC(-1, -1, -1, 1, 0, 0, 1),
		PC(1, -1, -1, 1, 0, 0, 1),
		PC(1, 1, -1, 1, 0, 0, 1),
		PC(1, -1, 1, 1, 0, 0, 1),
		PC(-1, 1, -1, 1, 0, 0, 1),
	}...)
	WhiteCube.SetTypes(gl.LINE_LOOP)
	RedCube.SetTypes(gl.LINE_LOOP)
	WhiteCube.GenVao()
	RedCube.GenVao()
	metaRaw := make([]byte, 1)
	_, err = c.Read(metaRaw)
	fmt.Println(metaRaw)
	orDie(err)
	lenInt = metaRaw[0] / 8
	fmt.Println("lenInt")
	fmt.Println(lenInt)
	switch lenInt {
	case 1:
		bytesToU64 = func(a []byte) uint64 { return uint64(a[0]) }
	case 2:
		bytesToU64 = func(a []byte) uint64 { return uint64(endianness.Uint16(a)) }
	case 4:
		bytesToU64 = func(a []byte) uint64 { return uint64(endianness.Uint32(a)) }
	case 8:
		bytesToU64 = endianness.Uint64
	}
	metaRaw = make([]byte, 3+3*lenInt)
	_, err = c.Read(metaRaw)
	orDie(err)
	maxWorldX, maxWorldY, maxWorldZ = parseCoords(metaRaw[0 : 3*lenInt])
	worldR, worldG, worldB := parseColors(metaRaw[3*lenInt : 3+3*lenInt])
	gl.ClearColor(worldR, worldG, worldB, 1)
	keysPressed = make(keyStack, 0)
	for !window.ShouldClose() {
		time.Sleep(fps)
		// Clear everything that was drawn previously
		gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)
		toBeRendered := handleKeys(keysPressed, c, window)
		// Actually draw something
		//		b.Draw()
		framesDrawn++
		fmt.Fprintf(os.Stderr, "Snake kitna lamba hai: %d\n", len(toBeRendered)-1)
		for _, v := range toBeRendered {
			UpdateUniformVec4("current_color", program, &v.C[0])
			WhiteCube.ModelMat = mgl32.Translate3D(v.X(), v.Y(), v.Z())
			WhiteCube.Draw()
			os.Stderr.WriteString("Here?")
		}
		// fnt.GlyphMap['e'].Draw()
		// display everything that was drawn
		window.SwapBuffers()
		// check for any events
		glfw.PollEvents()
	}
}

func handleKeys(keys keyStack, c *websocket.Conn, w *glfw.Window) []*Point {
	to_be_rendered := make([]*Point, 0)
	for len(keys) > 0 {
		fmt.Println(len(keys))
		key := keys.Pop()
		fmt.Println(key)
		switch key {
		case glfw.KeyEscape:
			writeToWebsocket(c, []byte{'E'})
			w.SetShouldClose(true)
			w.Destroy()
			os.Exit(0)
		case glfw.KeyUp:
			writeToWebsocket(c, []byte{'x'})
		case glfw.KeyDown:
			writeToWebsocket(c, []byte{'X'})
		case glfw.KeyRight:
			writeToWebsocket(c, []byte{'z'})

		case glfw.KeyLeft:
			writeToWebsocket(c, []byte{'Z'})

		case glfw.KeySpace:
			writeToWebsocket(c, []byte{'y'})

		case glfw.KeyZ:
			writeToWebsocket(c, []byte{'Y'})

		default:
			writeToWebsocket(c, []byte{'F'})
		}
		to_be_rendered = append(to_be_rendered, NextFrame(c)...)
	}
	return to_be_rendered
}

func writeToWebsocket(conn *websocket.Conn, data []byte) {
	writer, err := conn.NewFrameWriter(websocket.BinaryFrame)
	orDie(err)
	writer.Write(data)
}

func NextFrame(c *websocket.Conn) []*Point {
	dataRaw := make([]byte, 4)
	dataReader, err := c.NewFrameReader()
	orDie(err)
	_, err = dataReader.Read(dataRaw)
	orDie(err)
	fmt.Println(dataRaw)
	orDie(err)
	numPoints := binary.BigEndian.Uint32(dataRaw[0:4])
	if numPoints == 0 {
		fmt.Println("Game Over")
		scoreRaw := make([]byte, 4)
		_, err := dataReader.Read(scoreRaw)
		orDie(err)
		score = binary.BigEndian.Uint32(scoreRaw)
		gameEnded = true
	}
	points := make([]*Point, numPoints)
	dataRaw = make([]byte, 4+int(numPoints)*int(3*lenInt+3))
	for i := 0; i < int(numPoints); i++ {
		pointRaw := dataRaw[4+i*3*int(lenInt) : 4+i*int(3*lenInt+3)]
		points[i] = parsePoint(pointRaw)
	}
	return points
}

func parseCoords(size []byte) (float32, float32, float32) {
	x := bytesToU64(size[0:lenInt])
	y := bytesToU64(size[lenInt : 2*lenInt])
	z := bytesToU64(size[2*lenInt : 3*lenInt])
	return float32(x), float32(y), float32(z)
}

func parseColors(colors []byte) (float32, float32, float32) {
	r := float32(colors[0]) / 255
	g := float32(colors[1]) / 255
	b := float32(colors[2]) / 255
	return r, g, b
}

func parsePoint(point []byte) *Point {
	x, y, z := parseCoords(point[0 : 3*lenInt])
	r, g, b := parseColors(point[3*lenInt : 3*lenInt+3])
	return PC(x, y, z, r, g, b, 1)
}
