package emuios

import (
	ios "github.com/user-none/eblitui-ios"
	"github.com/user-none/ecolr/adapter"
)

func init() {
	ios.RegisterFactory(&adapter.Factory{})
}

// Re-export bridge functions for gomobile binding

func Init(path string, regionCode int) bool { return ios.Init(path, regionCode) }
func Close()                                { ios.Close() }
func RunFrame()                             { ios.RunFrame() }
func GetFrameData() []byte                  { return ios.GetFrameData() }
func GetAudioData() []byte                  { return ios.GetAudioData() }
func SetInput(player int, buttons int)      { ios.SetInput(player, buttons) }
func FrameWidth() int                       { return ios.FrameWidth() }
func FrameStride() int                      { return ios.FrameStride() }
func FrameHeight() int                      { return ios.FrameHeight() }
func SystemInfoJSON() string                { return ios.SystemInfoJSON() }
func Region() int                           { return ios.Region() }
func GetFPS() int                           { return ios.GetFPS() }
func DetectRegionFromPath(path string) int  { return ios.DetectRegionFromPath(path) }
func HasSaveStates() bool                   { return ios.HasSaveStates() }
func SaveState() bool                       { return ios.SaveState() }
func StateLen() int                         { return ios.StateLen() }
func StateByte(i int) int                   { return ios.StateByte(i) }
func LoadState(data []byte) bool            { return ios.LoadState(data) }
func HasSRAM() bool                         { return ios.HasSRAM() }
func PrepareSRAM()                          { ios.PrepareSRAM() }
func SRAMLen() int                          { return ios.SRAMLen() }
func SRAMByte(i int) int                    { return ios.SRAMByte(i) }
func LoadSRAM(data []byte)                  { ios.LoadSRAM(data) }
func ExtractAndStoreROM(srcPath, destDir string) (string, error) {
	return ios.ExtractAndStoreROM(srcPath, destDir)
}
func GetCRC32FromPath(path string) int64 { return ios.GetCRC32FromPath(path) }
func SetOption(key string, value string) { ios.SetOption(key, value) }
func Start()                             { ios.Start() }
