import AresClientFHE

public enum Rider {
    public static func keygen(ringDim: UInt32 = 1 << 15, depth: UInt32 = 5,
        scalingFactor: Double = Double(UInt64(1) << 50)) throws -> Keypair {
        let ctx = try CryptoContext(ringDim: ringDim, scalingFactor: scalingFactor, depth: depth)
        let kp = try ctx.singleKeyGen()
        let pk = try ctx.serialize(kp.publicKey)
        let sk = try ctx.serialize(kp.secretKey)
        return Keypair(publicKey: Data(pk), secretKey: Data(sk))
    }

    public static func decryptMasks(skBytes: Data, encryptedMasks: [Data],
        ringDim: UInt32 = 1 << 15, depth: UInt32 = 5,
        scalingFactor: Double = Double(UInt64(1) << 50)) throws -> (winnerIndex: Int, maskValues: [Double]) {
        let ctx = try CryptoContext(ringDim: ringDim, scalingFactor: scalingFactor, depth: depth)
        let sk = try ctx.deserializeSecretKeyShare(Array(skBytes), lead: true)
        var masks = [Double](); var best = -1; var bestVal = 0.0
        for (i, ctBytes) in encryptedMasks.enumerated() {
            let ct = try ctx.deserializeCiphertext(Array(ctBytes))
            let vals = try ctx.decryptSingle(ct, with: sk, slots: 1)
            masks.append(vals[0])
            if best < 0 || vals[0] > bestVal { best = i; bestVal = vals[0] }
        }
        return (best, masks)
    }
}
