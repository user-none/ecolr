import Foundation
import Emulator
import EblituiIOS

/// Concrete bridge provider for the emulator core.
struct CoreBridgeProvider: EmulatorBridgeProvider {
    private static var cachedSystemInfo: SystemInfo?

    static var systemInfo: SystemInfo {
        if let cached = cachedSystemInfo {
            return cached
        }
        let json = EmuiosSystemInfoJSON()
        if let data = json.data(using: .utf8),
           let info = try? JSONDecoder().decode(SystemInfo.self, from: data) {
            cachedSystemInfo = info
            return info
        }
        fatalError("Failed to decode SystemInfo from Go bridge")
    }

    static func createEngine() -> EmulatorEngine {
        return CoreEmulatorEngine()
    }

    static func crc32(ofPath path: String) -> UInt32? {
        let result = EmuiosGetCRC32FromPath(path)
        if result < 0 {
            return nil
        }
        return UInt32(result)
    }

    static func detectRegion(path: String) -> Int {
        return EmuiosDetectRegionFromPath(path)
    }

    static func extractAndStoreROM(srcPath: String, destDir: String) -> ROMImportResult? {
        var error: NSError?
        let jsonString = EmuiosExtractAndStoreROM(srcPath, destDir, &error)
        if error != nil {
            return nil
        }
        guard let jsonData = jsonString.data(using: .utf8) else {
            return nil
        }
        struct ExtractResult: Decodable {
            let crc: String
            let name: String
        }
        guard let decoded = try? JSONDecoder().decode(ExtractResult.self, from: jsonData) else {
            return nil
        }
        return ROMImportResult(crc: decoded.crc, name: decoded.name)
    }
}
