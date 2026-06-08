// swift-tools-version:5.9
import PackageDescription

let package = Package(
    name: "rideshareswift",
    platforms: [.macOS(.v13), .iOS(.v16)],
    products: [.library(name: "RideshareCoreLib", targets: ["RideshareCoreLib"])],
    dependencies: [
        .package(path: "../../../ARES-core/clients/swift"),
    ],
    targets: [
        .target(name: "RideshareCoreLib", dependencies: [
            .product(name: "AresClientFHE", package: "swift"),
        ]),
        .testTarget(name: "RideshareCoreTests", dependencies: ["RideshareCoreLib"]),
    ]
)
