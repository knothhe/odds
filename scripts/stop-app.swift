// Request a normal application termination only for this exact installation.
import AppKit
import Foundation

let requested = URL(fileURLWithPath: CommandLine.arguments[1]).standardizedFileURL.resolvingSymlinksInPath()
for app in NSWorkspace.shared.runningApplications {
    guard let bundle = app.bundleURL?.standardizedFileURL.resolvingSymlinksInPath(), bundle == requested else { continue }
    if !app.terminate() {
        fputs("The target application refused the quit request.\n", stderr)
        exit(1)
    }
}
