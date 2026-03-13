import Foundation
import Emulator
import EblituiIOS

/// Concrete emulator engine wrapping the Go Emulator framework.
class CoreEmulatorEngine: EmulatorEngine {
    private(set) var isLoaded = false

    var fps: Int {
        return EmuiosGetFPS()
    }

    var hasSaveStates: Bool {
        return EmuiosHasSaveStates()
    }

    var hasSRAM: Bool {
        return EmuiosHasSRAM()
    }

    // MARK: - Initialization

    func loadROM(path: String) -> Bool {
        let regionCode = EmuiosDetectRegionFromPath(path)
        let success = EmuiosInit(path, regionCode)
        isLoaded = success
        return success
    }

    func loadROM(path: String, region: Int) -> Bool {
        let success = EmuiosInit(path, region)
        isLoaded = success
        return success
    }

    func unload() {
        EmuiosClose()
        isLoaded = false
    }

    // MARK: - Frame Execution

    func runFrame() {
        EmuiosRunFrame()
    }

    func getFrameBuffer() -> FrameData? {
        guard let data = EmuiosGetFrameData() else { return nil }
        return FrameData(pixels: Data(data), stride: EmuiosFrameStride(), activeHeight: EmuiosFrameHeight())
    }

    // MARK: - Audio

    func getAudioSamples() -> Data? {
        guard let data = EmuiosGetAudioData() else { return nil }
        return Data(data)
    }

    // MARK: - Input

    func setInput(player: Int, buttons: Int) {
        EmuiosSetInput(player, buttons)
    }

    // MARK: - Core Options

    func setOption(key: String, value: String) {
        EmuiosSetOption(key, value)
    }

    func start() {
        EmuiosStart()
    }

    // MARK: - Save States

    func serialize() -> Data? {
        guard hasSaveStates else { return nil }
        guard EmuiosSaveState() else { return nil }

        let len = EmuiosStateLen()
        guard len > 0 else { return nil }

        var bytes = [UInt8](repeating: 0, count: len)
        for i in 0..<len {
            bytes[i] = UInt8(EmuiosStateByte(i))
        }
        return Data(bytes)
    }

    func deserialize(data: Data) -> Bool {
        guard hasSaveStates else { return false }
        return EmuiosLoadState(data)
    }

    // MARK: - SRAM (Battery Save)

    func getSRAM() -> Data? {
        guard hasSRAM else { return nil }
        EmuiosPrepareSRAM()
        let len = EmuiosSRAMLen()
        guard len > 0 else { return nil }

        var bytes = [UInt8](repeating: 0, count: len)
        for i in 0..<len {
            bytes[i] = UInt8(EmuiosSRAMByte(i))
        }
        return Data(bytes)
    }

    func setSRAM(data: Data) {
        guard hasSRAM else { return }
        EmuiosLoadSRAM(data)
    }
}
