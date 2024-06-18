package main

import (
	"fmt"
	"math"
	"math/bits"
	"os"
	"strings"

	gsrpc "github.com/centrifuge/go-substrate-rpc-client/v4"
	"github.com/centrifuge/go-substrate-rpc-client/v4/signature"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
)

func main() {

	// Instantiate the API
	api, err := gsrpc.NewSubstrateAPI("https://westend-rpc.polkadot.io")
	if err != nil {
		panic(err)
	}

	meta, err := api.RPC.State.GetMetadataLatest()
	if err != nil {
		panic(err)
	}

	// Create a call, transferring 12345 units to account 5FsqGPa2ibqg243TuzfR4Rno9a7w2FSFXbzFmoFxVjMTNBbZ
	bob, err := types.NewMultiAddressFromHexAccountID("0xa8a62cdc29835a28cb4efc20f3e6f72ef3aed39f1f5ab00b577e43028c686802")
	if err != nil {
		panic(err)
	}

	amount := types.NewUCompactFromUInt(12345)

	c, err := types.NewCall(meta, "Balances.transfer_allow_death", bob, amount)
	if err != nil {
		panic(err)
	}

	// Create the extrinsic
	ext := types.NewExtrinsic(c)

	genesisHash, err := api.RPC.Chain.GetBlockHash(0)
	if err != nil {
		panic(err)
	}

	rv, err := api.RPC.State.GetRuntimeVersionLatest()
	if err != nil {
		panic(err)
	}

	latestBlockHeader, err := api.RPC.Chain.GetHeaderLatest()
	if err != nil {
		panic(err)
	}

	keyPair := keyPairFromSeedPhrase()

	key, err := types.CreateStorageKey(meta, "System", "Account", keyPair.PublicKey)
	if err != nil {
		panic(err)
	}

	var nonce uint32
	var accountInfo types.AccountInfo
	ok, err := api.RPC.State.GetStorageLatest(key, &accountInfo)
	if err != nil {
		panic(err)
	}
	if ok {
		nonce = uint32(accountInfo.Nonce)
	}

	o := types.SignatureOptions{
		BlockHash:          latestBlockHeader.ParentHash,
		Era:                getMortalEra(uint64(latestBlockHeader.Number) - 1),
		GenesisHash:        genesisHash,
		Nonce:              types.NewUCompactFromUInt(uint64(nonce)),
		SpecVersion:        rv.SpecVersion,
		Tip:                types.NewUCompactFromUInt(0),
		TransactionVersion: rv.TransactionVersion,
	}

	fmt.Printf("Sending %d from %#x to %#x with nonce %v\n", amount.Int64(), keyPair.PublicKey, bob.AsID, nonce)

	signedExt := ext
	// Sign the transaction
	err = signedExt.Sign(keyPair, o)
	if err != nil {
		panic(err)
	}

	// submit signed extrinsic
	// Do the transfer and track the actual status
	hash, err := api.RPC.Author.SubmitExtrinsic(signedExt)
	if err != nil {
		panic(err)
	}

	hex := hash.Hex()
	fmt.Println("tx hash gotten from network:", hex)

}

func loadPhrase() string {
	bs, err := os.ReadFile("phrase.txt")
	if err != nil {
		panic(err)
	}

	//phrase2 := string(bs)
	spl := strings.Split(string(bs), "\n")
	phrase := spl[0]
	return phrase
}

func keyPairFromSeedPhrase() signature.KeyringPair {
	// load seed phrase
	phrase := loadPhrase()

	keyPair, err := signature.KeyringPairFromSecret(phrase, 42)
	if err != nil {
		panic(err)
	}

	return keyPair
}

const validityPeriod = 50

func getMortalEra(eraBirthBlockNumber uint64) types.ExtrinsicEra {
	period, phase := mortal(validityPeriod, eraBirthBlockNumber)

	return types.ExtrinsicEra{
		IsImmortalEra: false,
		IsMortalEra:   true,
		AsMortalEra:   newMortalEra(period, phase),
	}
}

// newMortalEra encodes a mortal era based on period and phase
func newMortalEra(period, phase uint64) types.MortalEra {
	quantizeFactor := math.Max(float64(period>>12), 1)

	trailingZeros := bits.TrailingZeros16(uint16(period))
	encoded := uint16(float64(phase)/quantizeFactor)<<4 | uint16(math.Min(15, math.Max(1, float64(trailingZeros-1))))

	return types.MortalEra{First: byte(encoded & 0xff), Second: byte(encoded >> 8)}
}

// mortal describes a mortal era based on a period of validity and a block number on which it should start
func mortal(validityPeriod, eraBirthBlockNumber uint64) (period, phase uint64) {
	calPeriod := math.Pow(2, math.Ceil(math.Log2(float64(validityPeriod))))
	period = uint64(math.Min(math.Max(calPeriod, 4), 1<<16))

	quantizeFactor := math.Max(float64(period>>12), 1)
	quantizedPhase := float64(eraBirthBlockNumber%period) / quantizeFactor * quantizeFactor

	phase = uint64(quantizedPhase)

	return
}
