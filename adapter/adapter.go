package adapter

import (
	"github.com/user-none/eblitui/coreif"
	"github.com/user-none/ecolr"
	"github.com/user-none/ecolr/core"
)

// Factory implements CoreFactory for the NGPC emulator.
type Factory struct{}

// NewFactory creates a Factory.
func NewFactory() *Factory {
	return &Factory{}
}

// SystemInfo returns system metadata for UI configuration.
func (f *Factory) SystemInfo() coreif.SystemInfo {
	return coreif.SystemInfo{
		Name:        "ecolr",
		ConsoleName: "Neo Geo Pocket Color",
		Extensions:  []string{".ngp", ".ngc"},
		ScreenWidth: core.ScreenWidth,
		// NGPC display is 160x152, 1:1 pixel aspect ratio (handheld LCD)
		MaxScreenHeight:  core.MaxScreenHeight,
		PixelAspectRatio: 1.0,
		SampleRate:       48000,
		Buttons: []coreif.Button{
			{Name: "A", ID: 4, DefaultKey: "J", DefaultPad: "A"},
			{Name: "B", ID: 5, DefaultKey: "K", DefaultPad: "B"},
			{Name: "Option", ID: 7, DefaultKey: "Enter", DefaultPad: "Start"},
		},
		Players: 1,
		CoreOptions: []coreif.CoreOption{
			{
				Key:         "first_boot",
				Label:       "First Boot",
				Description: "Run BIOS setup screens (language and color selection)",
				Type:        coreif.CoreOptionBool,
				Default:     "false",
				Category:    coreif.CoreOptionCategoryCore,
			},
			{
				Key:         "fast_boot",
				Label:       "Fast Boot",
				Description: "Skip BIOS animation screens and boot directly to game",
				Type:        coreif.CoreOptionBool,
				Default:     "true",
				Category:    coreif.CoreOptionCategoryCore,
			},
			{
				Key:         "mono_palette",
				Label:       "Monochrome Palette",
				Description: "Color scheme for monochrome (NGP) games",
				Type:        coreif.CoreOptionSelect,
				Default:     "Black & White",
				Values:      []string{"Black & White", "Red", "Green", "Blue", "Classic"},
				Category:    coreif.CoreOptionCategoryVideo,
			},
			{
				Key:         "language",
				Label:       "Language",
				Description: "System language",
				Type:        coreif.CoreOptionSelect,
				Default:     "English",
				Values:      []string{"English", "Japanese"},
				Category:    coreif.CoreOptionCategoryCore,
			},
		},
		BIOSOptions: []coreif.BIOSOption{
			{
				Key:      "system_bios",
				Label:    "System BIOS",
				Required: false,
				Variants: []coreif.BIOSVariant{
					{Label: "NGPC BIOS (JE)", SHA256: "8fb845a2f71514cec20728e2f0fecfade69444f8d50898b92c2259f1ba63e10d", Filename: "ngpc_bios.bin"},
				},
			},
		},
		MetadataVariants: []coreif.MetadataVariant{
			{Name: "Neo Geo Pocket Color", RDBName: "SNK - Neo Geo Pocket Color", ThumbnailRepo: "SNK_-_Neo_Geo_Pocket_Color"},
			{Name: "Neo Geo Pocket", RDBName: "SNK - Neo Geo Pocket", ThumbnailRepo: "SNK_-_Neo_Geo_Pocket"},
		},
		DataDirName:   "ecolr",
		ConsoleID:     14,
		CoreName:      ecolr.Name,
		CoreVersion:   ecolr.Version,
		SerializeSize: core.SerializeSize(),
	}
}

// CreateEmulator creates a new emulator instance with the given ROM.
// The emulator starts in HLE mode; call SetBIOS before Start() to use a real BIOS.
func (f *Factory) CreateEmulator(rom []byte) (coreif.Emulator, error) {
	e, err := core.NewEmulator(rom)
	if err != nil {
		return nil, err
	}
	return &e, nil
}
