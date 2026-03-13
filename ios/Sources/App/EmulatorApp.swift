import SwiftUI
import EblituiIOS

@main
struct EmulatorApp: App {
    @StateObject private var appState: AppState

    init() {
        EmulatorBridge.register(CoreBridgeProvider.self)
        _appState = StateObject(wrappedValue: AppState())
    }

    var body: some Scene {
        WindowGroup {
            ContentView()
                .environmentObject(appState)
                .preferredColorScheme(.dark)
        }
    }
}
