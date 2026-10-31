// Export one explicitly selected identity; passwords arrive only through stdin.
import CryptoKit
import Foundation
import Security

func fail(_ message: String) -> Never {
    fputs(message + "\n", stderr)
    exit(1)
}

guard CommandLine.arguments.count == 4 else {
    fail("Expected a keychain path, certificate SHA1 and private output path.")
}
let keychainPath = CommandLine.arguments[1]
let requested = CommandLine.arguments[2].uppercased()
let output = URL(fileURLWithPath: CommandLine.arguments[3])
guard requested.range(of: "^[A-F0-9]{40}$", options: .regularExpression) != nil,
      let password = String(data: FileHandle.standardInput.readDataToEndOfFile(), encoding: .utf8),
      !password.isEmpty else {
    fail("An exact identity and nonempty stdin password are required.")
}
var keychain: SecKeychain?
guard SecKeychainOpen(keychainPath, &keychain) == errSecSuccess, let keychain else {
    fail("Cannot open the selected keychain.")
}
let query: [CFString: Any] = [
    kSecClass: kSecClassIdentity,
    kSecMatchSearchList: [keychain],
    kSecMatchLimit: kSecMatchLimitAll,
    kSecReturnRef: true,
]
var found: CFTypeRef?
let status = SecItemCopyMatching(query as CFDictionary, &found)
guard status == errSecSuccess, let identities = found as? [SecIdentity] else {
    fail("Cannot list identities in the selected keychain (\(status)).")
}
var selected: [SecIdentity] = []
for identity in identities {
    var certificate: SecCertificate?
    guard SecIdentityCopyCertificate(identity, &certificate) == errSecSuccess,
          let certificate else {
        fail("Cannot read an identity certificate.")
    }
    let data = SecCertificateCopyData(certificate) as Data
    let digest = Insecure.SHA1.hash(data: data).map { String(format: "%02X", $0) }.joined()
    if digest == requested {
        selected.append(identity)
    }
}
guard selected.count == 1 else {
    fail("The selected keychain must contain exactly one matching identity.")
}

// SecItemExport receives only the selected SecIdentity, never a keychain or
// an array of identities. Other identities' private keys are never exported.
var parameters = SecItemImportExportKeyParameters()
parameters.version = UInt32(SEC_KEY_IMPORT_EXPORT_PARAMS_VERSION)
let passphrase = password as CFString
parameters.passphrase = Unmanaged.passUnretained(passphrase)
var exported: CFData?
let exportStatus = withExtendedLifetime(passphrase) {
    SecItemExport(selected[0], .formatPKCS12, [], &parameters, &exported)
}
guard exportStatus == errSecSuccess, let exported else {
    fail("Selected identity export failed (\(exportStatus)).")
}
do {
    let attributes = try FileManager.default.attributesOfItem(atPath: output.path)
    guard attributes[.type] as? FileAttributeType == .typeRegular,
          (attributes[.posixPermissions] as? NSNumber)?.intValue == 0o600 else {
        fail("The output must be an existing private regular file.")
    }
    let stream = try FileHandle(forWritingTo: output)
    defer { try? stream.close() }
    try stream.truncate(atOffset: 0)
    try stream.write(contentsOf: exported as Data)
} catch {
    fail("Cannot write the selected identity to its private output file.")
}
print("{\"exported\":true,\"certificateSHA1\":\"\(requested)\"}")
