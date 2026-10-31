// Renders an SVG to a PNG with a transparent background using AppKit.
// Usage: swift scripts/svg2png.swift <in.svg> <out.png> [size]

import AppKit
let args = CommandLine.arguments
let size = Int(args[3]) ?? 1024
guard let img = NSImage(contentsOf: URL(fileURLWithPath: args[1])) else { fatalError("cannot load svg") }
let rep = NSBitmapImageRep(bitmapDataPlanes: nil, pixelsWide: size, pixelsHigh: size, bitsPerSample: 8,
    samplesPerPixel: 4, hasAlpha: true, isPlanar: false, colorSpaceName: .deviceRGB, bytesPerRow: 0, bitsPerPixel: 0)!
NSGraphicsContext.saveGraphicsState()
NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep: rep)
NSColor.clear.set()
NSRect(x: 0, y: 0, width: size, height: size).fill()
img.draw(in: NSRect(x: 0, y: 0, width: size, height: size))
NSGraphicsContext.restoreGraphicsState()
try! rep.representation(using: .png, properties: [:])!.write(to: URL(fileURLWithPath: args[2]))
