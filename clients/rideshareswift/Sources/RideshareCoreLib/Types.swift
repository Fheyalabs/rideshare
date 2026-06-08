import Foundation

public struct Keypair {
    public let publicKey: Data
    public let secretKey: Data
    public init(publicKey: Data, secretKey: Data) {
        self.publicKey = publicKey; self.secretKey = secretKey
    }
}

public struct EncryptedBid {
    public let ciphertext: Data; public let priceCents: Int; public let nonce: Data
    public init(ciphertext: Data, priceCents: Int, nonce: Data) {
        self.ciphertext = ciphertext; self.priceCents = priceCents; self.nonce = nonce
    }
}
