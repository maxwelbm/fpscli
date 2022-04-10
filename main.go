// faithful of JavidX9's CommandLineFPS
// (https://github.com/OneLoneCoder/CommandLineFPS) from C++ to Go

package main

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"sort"
	"time"

	"github.com/gdamore/tcell/v2"
)

const debug = false // display coordinates and other info at top of screen

// Note nScreenWidth and nScreenHeight are not defined here: tcell does not
// allow fixed screen sizes, so these are determined dynamically in main()
// below.

// Added a couple of convenience constants:
const pi = 3.14159
const tau = 2 * pi

const nMapWidth = 16 // world dimensions
const nMapHeight = 16

var fPlayerX = 14.4              // player x position
var fPlayerY = 14.7              // player y position
var fPlayerA = pi                // player angle
const fFOV = pi / 4              // field of view
const fDepth = 16.0              // maximum rendering distance
const fSpeed = 9.0               // walking speed
const fTurnSpeed = fSpeed * 0.75 // added convenience constant

// Introducing a tick to fix frame rate at 60s; this appears to have been
// unnecessary with c++ chrono library in original, but is needed here to keep
// things smooth
const tick = 15 * time.Millisecond

var screen tcell.Screen
var err error

// JavidX9's video made the walls seem grey rather than white, so I'm doing a
// similarly dark color for the walls and floor:
var mazeStyle = tcell.StyleDefault.Background(tcell.ColorBlack).
	Foreground(tcell.ColorDarkSlateBlue)

// Let's add a little something new: a moon and starry sky!
var skyStyle = tcell.StyleDefault.Background(tcell.ColorBlack).
	Foreground(tcell.ColorWhite)
var moonStyle = tcell.StyleDefault.Background(tcell.ColorBlack).
	Foreground(tcell.ColorPaleGoldenrod)

// The moon sprite in unicode runes. The rune '@' denotes invisible (a fake
// alpha channel)
var moon = [6][]rune{
	[]rune("@@@██████@@@"),
	[]rune("@▓▓▓▓▓█▓▓██@"),
	[]rune("▓▓▓▓███▓▓▓█▓"),
	[]rune("▓▓██▓████▓██"),
	[]rune("@▓▓▓▓▓█████@"),
	[]rune("@@@██████@@@"),
}

const nMoonWidth = 12
const nMoonHeight = 6
const fMoonA = pi // moon's angle in the sky

func main() {

	// create screen buffer
	if screen, err = tcell.NewScreen(); err != nil {
		fmt.Println("Failed to start tcell")
		os.Exit(1)
	}
	err = screen.Init()
	if err != nil {
		fmt.Println("Failed to init tcell.Screen")
		os.Exit(1)
	}

	screen.HideCursor()
	screen.SetStyle(skyStyle)
	screen.Clear()
	// get the screen width and height (determined at runtime by tcell):
	nScreenWidth, nScreenHeight := screen.Size()

	// Detour from JavidX9's original to make the sky! We'll use a simple
	// cylindrical projection and a randomly-generated starfield. The projection
	// uses the field of view to calculate an apparent radius and then
	// circumference for the sky
	nSkyHeight := nScreenHeight / 2                    // horizon to top
	fSkyApparentRadius := float64(nScreenWidth) / fFOV // r = s/θ, where θ is the field of view
	nSkyCircumference := int(tau * fSkyApparentRadius) // C = 2πr
	nMoonStartX := int(fSkyApparentRadius * fMoonA)    // s = rθ, the x offset of the moon in the sky
	nMoonStartY := 1                                   // close to the top so visible from most places in the maze
	// make the sky:
	sky := make([][]rune, nSkyCircumference)
	moonCoords := make([][2]int, 0)
	for x := 0; x < nSkyCircumference; x++ {
		sky[x] = make([]rune, nSkyHeight)
		for y := 0; y < nSkyHeight; y++ {
			// first, determine if we should draw moon here and, if so, fetch
			// appropriate moon rune
			var rMoonShade rune
			bIsMoon := false
			bInMoonRange := x >= nMoonStartX && x < nMoonStartX+nMoonWidth &&
				y >= nMoonStartY && y < nMoonStartY+nMoonHeight
			if bInMoonRange {
				nMoonX := x - nMoonStartX
				nMoonY := y - nMoonStartY
				rMoonShade = moon[nMoonY][nMoonX]
				bIsMoon = rMoonShade != '@' // not invisible/alpha
			}
			switch {
			case x == 0 && debug:
				sky[x][y] = '|' // show line in sky at 0 radians
			case bIsMoon:
				sky[x][y] = rMoonShade
				moonCoords = append(moonCoords, [2]int{x, y})
			case rand.Float64() < 0.02: // a scattering of stars
				sky[x][y] = '.'
			default:
				sky[x][y] = ' ' // the ebon void
			}
		}
	}

	// Back to translating JavidX9's original code: create map of world where
	// '#' == wall, '.' == space
	const worldMap = "" +
		"#########......." +
		"#..............." +
		"#.......########" +
		"#..............#" +
		"#......##......#" +
		"#......##......#" +
		"#..............#" +
		"###............#" +
		"##.............#" +
		"#......####..###" +
		"#......#.......#" +
		"#......#.......#" +
		"#..............#" +
		"#......#########" +
		"#..............." +
		"################"

	// start ticker and timing
	ticker := time.NewTicker(tick)
	tp1 := time.Now()
	tp2 := time.Now()

	for {
		// this, which OLC used to ensure consistent movement, proved
		// insufficient (jerky), so I added a ticker. Kept this to caculate
		// actual framerate
		tp2 = time.Now()
		fElapsedTime := tp2.Sub(tp1).Seconds()
		tp1 = tp2

		// Check for an player input (note use of tick.Seconds() instead of
		// fElapsedTime as discussed above)
		switch event := screen.PollEvent().(type) {
		case *tcell.EventKey:
			switch {
			case event.Key() == tcell.KeyEscape:
				// Quit
				screen.Fini()
				os.Exit(0)
			case event.Key() == tcell.KeyLeft:
				// CCW rotation. Note a small difference here from JavidX9's
				// original: because we need to use the player's angle to draw
				// the sky properly (with moon and stars fixed under rotation),
				// we have to keep it bounded between 0 and 2π, rather than
				// letting it go negative or run off to infinity
				angle := fPlayerA - fTurnSpeed*tick.Seconds()
				fPlayerA = angle - tau*math.Floor(angle/tau) // mod 2π
			case event.Key() == tcell.KeyRight:
				// CW rotation. Same as CCW rotation, we take the angle mod 2π
				angle := fPlayerA + fTurnSpeed*tick.Seconds()
				fPlayerA = angle - tau*math.Floor(angle/tau)
			case event.Key() == tcell.KeyUp:
				// Forward movement and collision
				fPlayerX += math.Sin(fPlayerA) * fSpeed * tick.Seconds()
				fPlayerY += math.Cos(fPlayerA) * fSpeed * tick.Seconds()
				nMapIndex := int(fPlayerX)*nMapWidth + int(fPlayerY)
				if nMapIndex < 0 || nMapIndex >= len(worldMap) || // we add extra check for out of map bounds
					worldMap[nMapIndex] == '#' {
					// collision; seems odd to first move into the wall above,
					// then back out here, but that's how the original does it
					fPlayerX -= math.Sin(fPlayerA) * fSpeed * tick.Seconds()
					fPlayerY -= math.Cos(fPlayerA) * fSpeed * tick.Seconds()
				}
			case event.Key() == tcell.KeyDown:
				// Backward movement and collision
				fPlayerX -= math.Sin(fPlayerA) * fSpeed * tick.Seconds()
				fPlayerY -= math.Cos(fPlayerA) * fSpeed * tick.Seconds()
				nMapIndex := int(fPlayerX)*nMapWidth + int(fPlayerY)
				if nMapIndex < 0 || nMapIndex >= len(worldMap) || // we add extra check for out of map bounds
					worldMap[nMapIndex] == '#' {
					fPlayerX += math.Sin(fPlayerA) * fSpeed * tick.Seconds()
					fPlayerY += math.Cos(fPlayerA) * fSpeed * tick.Seconds()
				}
			}
		}

		for x := 0; x < nScreenWidth; x++ {
			// Loop over text columns

			// Calculate the projected ray angle into the world
			fRayAngle := (fPlayerA - fFOV/2.0) + (float64(x) / float64(nScreenWidth) * fFOV)

			// Find distance to wall
			fStepSize := 0.1 // for ray casting, decrease to increase resolution
			fDistanceToWall := 0.0

			bHitWall := false  // set when ray hits a wall block
			bBoundary := false // set when ray hits boundary between two wall blocks

			fEyeX := math.Sin(fRayAngle) // unit vector for ray
			fEyeY := math.Cos(fRayAngle)

			// Cast ray from player, along ray angle, testing for entry into a
			// wall block at intervals determined by step size. As JavidX9
			// noted, this is only the most efficient algorithm if you happen to
			// be close to a wall
			for !bHitWall && fDistanceToWall < fDepth {
				fDistanceToWall += fStepSize
				nTestX := int(fPlayerX + fEyeX*fDistanceToWall)
				nTestY := int(fPlayerY + fEyeY*fDistanceToWall)

				// Test for a step into a wall
				if nTestX < 0 || nTestX >= nMapWidth || nTestY < 0 || nTestY >= nMapHeight {
					bHitWall = true
					fDistanceToWall = fDepth
				} else if worldMap[nTestX*nMapWidth+nTestY] == '#' {
					bHitWall = true // folks, we hit a wall

					// And now a tricky part (present in the original), where we
					// check whether the ray we cast is "close" to a corner of
					// the wall block we hit, and, if it is, we'll shade it
					// differently to mark block boundaries. Here, "close" is
					// defined as the dot product of the cast ray and the ray
					// from the block corner to the player fitting within a
					// certain tolerance

					// As in the original, we'll store the distance from the
					// corner to the player, d, and the dot product of the
					// corner ray with the casting ray, dot, as a slice of
					// pairs: [][2]float64{d, dot}
					p := make([][2]float64, 0)
					for tx := 0; tx < 2; tx++ {
						for ty := 0; ty < 2; ty++ {
							vy := float64(nTestY) + float64(ty) - fPlayerY
							vx := float64(nTestX) + float64(tx) - fPlayerX
							d := math.Sqrt(vx*vx + vy*vy)
							dot := (fEyeX * vx / d) + (fEyeY * vy / d)
							p = append(p, [2]float64{d, dot})
						}
					}

					// Sort pairs from closest to farthest
					sort.Slice(p, func(i, j int) bool { return p[i][0] < p[j][0] })

					fBound := 0.01 // tolerance to be considered a corner hit

					// Check the first two/three corners: we'll never see all
					// four. As JavidX9 notes in the video, this does lead
					// occasionally to viewing corners that should be obscured
					// by a block face. We can fix this in a later revision
					switch {
					case math.Acos(p[0][1]) < fBound:
						bBoundary = p[0][0] < fDistanceToWall
					case math.Acos(p[1][1]) < fBound:
						bBoundary = p[1][0] < fDistanceToWall
					case math.Acos(p[2][1]) < fBound:
						bBoundary = p[2][0] < fDistanceToWall
					}
				}
			}

			// Calculate distance to ceiling (which we made a sky) and floor
			nCeiling := float64(nScreenHeight)/2.0 - float64(nScreenHeight)/fDistanceToWall
			nFloor := float64(nScreenHeight) - nCeiling

			var rShade rune // nShade in the original
			switch {
			case bBoundary == true:
				rShade = ' ' // black out wall block boundary
			case fDistanceToWall <= fDepth/3.0: // close, bright
				rShade = '█'
			case fDistanceToWall <= fDepth/2.0:
				rShade = '▓'
			case fDistanceToWall <= fDepth/1.1: // far, dark
				rShade = '░'
			default:
				rShade = ' ' // too far away, black
			}

			// Draw the screen!
			for y := 0; y < nScreenHeight; y++ {
				fY := float64(y)
				switch {
				case fY <= nCeiling:
					// Sky!
					angle := fPlayerA - pi/8
					angle = angle - tau*math.Floor(angle/tau)
					nPlayerAOffset := (x + int(fSkyApparentRadius*angle)) % nSkyCircumference
					style := skyStyle // for stars
					for _, coord := range moonCoords {
						if [2]int{nPlayerAOffset, y} == coord {
							style = moonStyle // moon!
							break
						}
					}
					screen.SetContent(x, y, sky[nPlayerAOffset][y], nil, style)
				case fY > nCeiling && fY <= nFloor:
					screen.SetContent(x, y, rShade, nil, mazeStyle)
				default:
					// Floor, shaded by distance from player
					b := 1.0 - (float64(y)-float64(nScreenHeight)/2.0)/(float64(nScreenHeight)/2.0)
					switch {
					case b < 0.25:
						rShade = '#'
					case b < 0.5:
						rShade = 'x'
					case b < 0.75:
						rShade = '.'
					case b < 0.9:
						rShade = '-'
					default:
						rShade = ' '
					}
					screen.SetContent(x, y, rShade, nil, mazeStyle)
				}
			}
		}
		if debug {
			// Display stats
			stats := fmt.Sprintf("X=%3.2f, Y=%3.2f, A=%3.2f, FPS=%3.2f, W=%v, C=%v, R=%v", fPlayerX, fPlayerY, fPlayerA, 1.0/fElapsedTime, nScreenWidth, nSkyCircumference, fSkyApparentRadius)
			for i, c := range stats {
				screen.SetContent(i, 0, c, nil, mazeStyle)
			}
		}

		screen.Show()

		<-ticker.C // wait for tick
	}
}
