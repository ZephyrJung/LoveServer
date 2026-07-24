package tankbattle

// Screen dimensions
const ScreenWidth = 800
const ScreenHeight = 600

// Tank constants
const TankSpeed = 100          // pixels/sec
const TankBodyW = 30
const TankBodyH = 40
const TankHeadRadius = 12
const TankCannonW = 4
const TankCannonH = 30
const TankMaxHealth = 3
const TankRespawnTime = 3.0    // seconds

// Bullet constants
const BulletSpeed = 300        // pixels/sec
const BulletRadius = 3
const BulletCooldown = 0.5     // seconds

// Wall constants
const WallThickness = 16
const BrickW = 32
const BrickH = 16
const SteelW = 32
const SteelH = 16

// Network
const TickRate = 20            // Hz
const TickDuration = 1.0 / TickRate // 0.05s

// Game rules
const DMKillLimit = 10
const TeamKillLimit = 15

// Max players
const MinPlayers = 2
const MaxPlayers = 6