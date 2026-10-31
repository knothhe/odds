// Read registrations without asking LaunchServices to scan or change a bundle.
import CoreServices
import Foundation

guard CommandLine.arguments.count == 3 else {
    fputs("Expected a bundle identifier and an exact application path.\n", stderr)
    exit(1)
}
let requested = URL(fileURLWithPath: CommandLine.arguments[2]).standardizedFileURL.resolvingSymlinksInPath()
var queryError: Unmanaged<CFError>?
let registered: Bool
if let applications = LSCopyApplicationURLsForBundleIdentifier(CommandLine.arguments[1] as CFString, &queryError) {
    let urls = applications.takeRetainedValue() as! [URL]
    registered = urls.contains { $0.standardizedFileURL.resolvingSymlinksInPath() == requested }
} else if let error = queryError?.takeRetainedValue() {
    guard CFErrorGetDomain(error) as String == NSOSStatusErrorDomain,
          CFErrorGetCode(error) == Int(kLSApplicationNotFoundErr) else {
        fputs("LaunchServices registration query failed (\(CFErrorGetCode(error))).\n", stderr)
        exit(1)
    }
    registered = false
} else {
    fputs("LaunchServices registration query returned neither applications nor an error.\n", stderr)
    exit(1)
}
print(registered ? "{\"registered\":true}" : "{\"registered\":false}")
