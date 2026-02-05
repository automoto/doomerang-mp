package systems

import (
	"math"

	astar "github.com/beefsack/go-astar"
	"github.com/solarlune/resolv"

	cfg "github.com/automoto/doomerang-mp/config"
	"github.com/automoto/doomerang-mp/tags"
)

// NavGrid represents the walkable areas of the level
type NavGrid struct {
	Width, Height int
	CellSize      float64
	Nodes         [][]*NavNode // 2D grid of nodes
}

// NavNode represents a single cell in the navigation grid
// Implements astar.Pather interface
type NavNode struct {
	X, Y     int
	Walkable bool     // Can stand/move through this cell
	Grid     *NavGrid // Reference to parent grid for neighbor lookup
}

// PathNeighbors returns adjacent walkable nodes (implements astar.Pather)
func (n *NavNode) PathNeighbors() []astar.Pather {
	var neighbors []astar.Pather

	// 8-directional movement (cardinal + diagonal)
	dirs := []struct{ dx, dy int }{
		{-1, 0}, {1, 0}, {0, -1}, {0, 1}, // Cardinal
		{-1, -1}, {1, -1}, {-1, 1}, {1, 1}, // Diagonal
	}

	for _, d := range dirs {
		nx, ny := n.X+d.dx, n.Y+d.dy

		// Check bounds
		if nx < 0 || nx >= n.Grid.Width || ny < 0 || ny >= n.Grid.Height {
			continue
		}

		neighbor := n.Grid.Nodes[ny][nx]
		if neighbor.Walkable {
			neighbors = append(neighbors, neighbor)
		}
	}

	// Add jump targets for platformer navigation
	jumpTargets := n.getJumpTargets()
	neighbors = append(neighbors, jumpTargets...)

	return neighbors
}

// PathNeighborCost returns the movement cost between adjacent nodes (implements astar.Pather)
func (n *NavNode) PathNeighborCost(to astar.Pather) float64 {
	toNode := to.(*NavNode)

	dx := float64(toNode.X - n.X)
	dy := float64(toNode.Y - n.Y)

	// Diagonal movement costs more (sqrt(2) ≈ 1.414)
	baseCost := math.Sqrt(dx*dx + dy*dy)

	// Penalize vertical movement (jumping is harder than walking)
	if dy < 0 { // Moving up requires a jump
		baseCost *= 1.5
	}

	return baseCost
}

// PathEstimatedCost returns heuristic distance to target (implements astar.Pather)
func (n *NavNode) PathEstimatedCost(to astar.Pather) float64 {
	toNode := to.(*NavNode)

	dx := float64(toNode.X - n.X)
	dy := float64(toNode.Y - n.Y)

	// Euclidean distance heuristic
	return math.Sqrt(dx*dx + dy*dy)
}

// getJumpTargets returns reachable nodes via jumping
// Uses physics values from config
func (n *NavNode) getJumpTargets() []astar.Pather {
	var targets []astar.Pather

	cellSize := n.Grid.CellSize
	maxJumpHeight := cfg.Pathfinding.MaxJumpHeight
	maxJumpDist := cfg.Pathfinding.MaxJumpDistance

	// Convert physics values to grid cells
	maxJumpHeightCells := int(maxJumpHeight/cellSize) - 1
	maxJumpDistCells := int(maxJumpDist/cellSize) - 1

	// Check potential landing spots
	for dy := -maxJumpHeightCells; dy <= 2; dy++ { // Can fall 2 cells down
		for dx := -maxJumpDistCells; dx <= maxJumpDistCells; dx++ {
			// Skip small movements (covered by regular neighbors)
			if absInt(dx) <= 1 && absInt(dy) <= 1 {
				continue
			}

			nx, ny := n.X+dx, n.Y+dy

			// Check bounds
			if nx < 0 || nx >= n.Grid.Width || ny < 0 || ny >= n.Grid.Height {
				continue
			}

			// Validate this jump is actually possible with physics
			if !n.isJumpReachable(dx, dy) {
				continue
			}

			target := n.Grid.Nodes[ny][nx]

			// Must be walkable (air) and have ground below (platform to land on)
			if target.Walkable && n.hasGroundBelow(nx, ny) {
				targets = append(targets, target)
			}
		}
	}

	return targets
}

// isJumpReachable checks if a jump from current position to (dx, dy) offset is physically possible
func (n *NavNode) isJumpReachable(dx, dy int) bool {
	cellSize := n.Grid.CellSize

	// Get physics values from config
	jumpSpeed := cfg.Player.JumpSpeed
	gravity := cfg.Physics.Gravity
	maxHorizontalSpd := cfg.Player.MaxSpeed
	maxJumpHeight := cfg.Pathfinding.MaxJumpHeight
	airTime := float64(cfg.Pathfinding.AirTime)

	// Convert to pixels
	horizontalDist := math.Abs(float64(dx)) * cellSize
	verticalDist := float64(dy) * cellSize // Negative = up, positive = down

	// Jumping UP (dy < 0): Check if we can reach that height
	if dy < 0 {
		heightNeeded := -verticalDist
		if heightNeeded > maxJumpHeight {
			return false // Can't jump that high
		}

		// Calculate time to reach that height
		discriminant := jumpSpeed*jumpSpeed - 2*gravity*heightNeeded
		if discriminant < 0 {
			return false
		}
		timeToHeight := (jumpSpeed - math.Sqrt(discriminant)) / gravity
		timeRemaining := airTime - timeToHeight

		// Can we cover horizontal distance in remaining air time?
		maxHorizontalAtHeight := maxHorizontalSpd * timeRemaining
		return horizontalDist <= maxHorizontalAtHeight
	}

	// Jumping DOWN or FLAT (dy >= 0): More forgiving
	fallTime := airTime
	if dy > 0 {
		// Extra time from falling: t = sqrt(2h/g)
		fallTime += math.Sqrt(2 * verticalDist / gravity)
	}

	maxHorizontal := maxHorizontalSpd * fallTime
	return horizontalDist <= maxHorizontal
}

// hasGroundBelow checks if there's solid ground beneath a position
func (n *NavNode) hasGroundBelow(x, y int) bool {
	// Check the cell below
	belowY := y + 1
	if belowY >= n.Grid.Height {
		return true // Bottom of level counts as ground
	}

	below := n.Grid.Nodes[belowY][x]
	return !below.Walkable // Non-walkable below = solid ground
}

// CreateNavGrid builds navigation grid from resolv Space
func CreateNavGrid(space *resolv.Space, levelWidth, levelHeight int, cellSize float64) *NavGrid {
	gridW := int(float64(levelWidth) / cellSize)
	gridH := int(float64(levelHeight) / cellSize)

	grid := &NavGrid{
		Width:    gridW,
		Height:   gridH,
		CellSize: cellSize,
		Nodes:    make([][]*NavNode, gridH),
	}

	// Initialize all nodes
	for y := 0; y < gridH; y++ {
		grid.Nodes[y] = make([]*NavNode, gridW)
		for x := 0; x < gridW; x++ {
			grid.Nodes[y][x] = &NavNode{
				X:        x,
				Y:        y,
				Walkable: true, // Default to walkable
				Grid:     grid,
			}
		}
	}

	// Mark cells as non-walkable based on collision data
	for y := 0; y < gridH; y++ {
		for x := 0; x < gridW; x++ {
			worldX := float64(x) * cellSize
			worldY := float64(y) * cellSize

			// Create a test object at this cell
			testObj := resolv.NewObject(worldX+2, worldY+2, cellSize-4, cellSize-4)
			space.Add(testObj)

			// Check if cell overlaps solid geometry
			if testObj.Check(0, 0, tags.ResolvSolid) != nil {
				grid.Nodes[y][x].Walkable = false
			}

			space.Remove(testObj)
		}
	}

	return grid
}

// FindPath uses go-astar to find path between world coordinates
func (g *NavGrid) FindPath(startX, startY, goalX, goalY float64) []*NavNode {
	// Convert world coords to grid coords
	sx := clampInt(int(startX/g.CellSize), 0, g.Width-1)
	sy := clampInt(int(startY/g.CellSize), 0, g.Height-1)
	gx := clampInt(int(goalX/g.CellSize), 0, g.Width-1)
	gy := clampInt(int(goalY/g.CellSize), 0, g.Height-1)

	startNode := g.Nodes[sy][sx]
	goalNode := g.Nodes[gy][gx]

	// Handle case where start or goal is in solid geometry
	if !startNode.Walkable {
		startNode = g.findNearestWalkable(sx, sy)
	}
	if !goalNode.Walkable {
		goalNode = g.findNearestWalkable(gx, gy)
	}

	if startNode == nil || goalNode == nil {
		return nil
	}

	// Use go-astar library to find path
	path, _, found := astar.Path(startNode, goalNode)
	if !found {
		return nil
	}

	// Convert []astar.Pather to []*NavNode
	result := make([]*NavNode, len(path))
	for i, p := range path {
		result[i] = p.(*NavNode)
	}

	return result
}

// findNearestWalkable finds the nearest walkable node to the given position
func (g *NavGrid) findNearestWalkable(x, y int) *NavNode {
	// Search in expanding squares
	for radius := 1; radius < 10; radius++ {
		for dy := -radius; dy <= radius; dy++ {
			for dx := -radius; dx <= radius; dx++ {
				nx, ny := x+dx, y+dy
				if nx >= 0 && nx < g.Width && ny >= 0 && ny < g.Height {
					if g.Nodes[ny][nx].Walkable {
						return g.Nodes[ny][nx]
					}
				}
			}
		}
	}
	return nil
}

// GridToWorld converts grid coordinates to world coordinates (center of cell)
func (g *NavGrid) GridToWorld(gridX, gridY int) (float64, float64) {
	return float64(gridX)*g.CellSize + g.CellSize/2,
		float64(gridY)*g.CellSize + g.CellSize/2
}

// Helper functions using Go 1.21+ builtins where possible
func clampInt(v, minVal, maxVal int) int {
	return max(minVal, min(maxVal, v))
}

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
