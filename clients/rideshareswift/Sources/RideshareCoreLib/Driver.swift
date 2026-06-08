import AresClientFHE

public enum Driver {
    public static func encryptBid(priceCents: Int, under pkBytes: Data,
        ringDim: UInt32 = 1 << 15, depth: UInt32 = 5,
        scalingFactor: Double = Double(UInt64(1) << 50)) throws -> EncryptedBid {
        let ctx = try CryptoContext(ringDim: ringDim, scalingFactor: scalingFactor, depth: depth)
        let pk = try ctx.deserializePublicKey(Array(pkBytes))
        let ct = try ctx.encrypt([Double(priceCents), 0, 0, 0], under: pk)
        let ctBytes = try ctx.serialize(ct)
        let nonce = Data((0..<16).map { _ in UInt8.random(in: 0...255) })
        return EncryptedBid(ciphertext: Data(ctBytes), priceCents: priceCents, nonce: nonce)
    }
}
