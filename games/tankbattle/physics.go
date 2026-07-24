package tankbattle

import "math"

// Rect represents an axis-aligned bounding box
type Rect struct {
	X, Y, W, H float64
}

// Circle represents a circle
type Circle struct {
	X, Y, R float64
}

// RectRectCollision checks AABB collision between two rectangles
func RectRectCollision(a, b Rect) bool {
	return a.X < b.X+b.W && a.X+a.W > b.X &&
		a.Y < b.Y+b.H && a.Y+a.H > b.Y
}

// CircleRectCollision checks collision between a circle and a rectangle
func CircleRectCollision(c Circle, r Rect) bool {
	closestX := math.Max(r.X, math.Min(c.X, r.X+r.W))
	closestY := math.Max(r.Y, math.Min(c.Y, r.Y+r.H))
	dx := c.X - closestX
	dy := c.Y - closestY
	return (dx*dx + dy*dy) < (c.R * c.R)
}

// CollidesWithWalls checks if a rectangle collides with any alive wall
func CollidesWithWalls(rect Rect, walls []*Wall) bool {
	for _, w := range walls {
		if w.Alive {
			if RectRectCollision(rect, Rect{X: w.X, Y: w.Y, W: w.W, H: w.H}) {
				return true
			}
		}
	}
	return false
}

// BulletCollidesWithWalls checks if a bullet circle collides with any wall.
// Returns the wall index if hit, or -1 if no collision.
func BulletCollidesWithWalls(c Circle, walls []*Wall) int {
	for i, w := range walls {
		if w.Alive {
			if CircleRectCollision(c, Rect{X: w.X, Y: w.Y, W: w.W, H: w.H}) {
				return i
			}
		}
	}
	return -1
}

// IsOutOfBounds checks if a point is outside the screen
func IsOutOfBounds(x, y float64) bool {
	return x < 0 || x > ScreenWidth || y < 0 || y > ScreenHeight
}