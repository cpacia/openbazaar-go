package core

import (
	"crypto/sha256"
	"errors"
	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"golang.org/x/net/context"
	multihash "gx/ipfs/QmYf7ng2hG5XBtJA3tN34DQ2GUN5HNksEw1rLDkmr6vGku/go-multihash"
	ma "gx/ipfs/QmYzDkkgAEmrcNzFCiYo6L1dTX4EAG1gZkbtdbd9trL4vd/go-multiaddr"
	"io/ioutil"
	"os"
	"path"
	"strings"
)

var ModeratorPointerID multihash.Multihash

func init() {
	modHash := sha256.Sum256([]byte("moderators"))
	encoded, err := multihash.Encode(modHash[:], multihash.SHA2_256)
	if err != nil {
		log.Fatal("Error creating moderator pointer ID (multihash encode)")
	}
	mh, err := multihash.Cast(encoded)
	if err != nil {
		log.Fatal("Error creating moderator pointer ID (multihash cast)")
	}
	ModeratorPointerID = mh
}

func (n *OpenBazaarNode) SetSelfAsModerator(moderator *pb.Moderator) error {
	if moderator.Fee == nil {
		return errors.New("Moderator must have a fee set")
	}
	if (int(moderator.Fee.FeeType) == 0 || int(moderator.Fee.FeeType) == 2) && moderator.Fee.FixedFee == nil {
		return errors.New("Fixed fee must be set when using a fixed fee type")
	}

	// Add bitcoin master public key
	mPubKey, err := n.Wallet.MasterPublicKey().ECPubKey()
	if err != nil {
		return err
	}
	moderator.PubKey = mPubKey.SerializeCompressed()

	// Save to file
	modPath := path.Join(n.RepoPath, "root", "moderation")
	m := jsonpb.Marshaler{
		EnumsAsInts:  false,
		EmitDefaults: true,
		Indent:       "    ",
		OrigName:     false,
	}
	out, err := m.MarshalToString(moderator)
	if err != nil {
		return err
	}
	f, err := os.Create(modPath)
	defer f.Close()
	if err != nil {
		return err
	}
	if _, err := f.WriteString(out); err != nil {
		return err
	}

	// Update profile
	profile, err := n.GetProfile()
	if err != nil {
		return err
	}
	profile.Moderator = true
	err = n.UpdateProfile(&profile)
	if err != nil {
		return err
	}

	// Publish pointer
	ctx := context.Background()

	b, err := multihash.Encode([]byte(n.IpfsNode.Identity.Pretty()), multihash.SHA1)
	if err != nil {
		return err
	}
	mhc, err := multihash.Cast(b)
	if err != nil {
		return err
	}
	addr, err := ma.NewMultiaddr("/ipfs/" + mhc.B58String())
	if err != nil {
		return err
	}
	pointer, err := ipfs.PublishPointer(n.IpfsNode, ctx, ModeratorPointerID, 64, addr)
	if err != nil {
		return err
	}
	pointer.Purpose = ipfs.MODERATOR
	err = n.Datastore.Pointers().DeleteAll(pointer.Purpose)
	if err != nil {
		return err
	}
	err = n.Datastore.Pointers().Put(pointer)
	if err != nil {
		return err
	}
	return nil
}

func (n *OpenBazaarNode) RemoveSelfAsModerator() error {
	// Update profile
	profile, err := n.GetProfile()
	if err != nil {
		return err
	}
	profile.Moderator = false
	err = n.UpdateProfile(&profile)
	if err != nil {
		return err
	}

	// Delete moderator file
	err = os.Remove(path.Join(n.RepoPath, "root", "moderation"))
	if err != nil {
		return err
	}

	// Delete pointer from database
	err = n.Datastore.Pointers().DeleteAll(ipfs.MODERATOR)
	if err != nil {
		return err
	}
	return nil
}

func (n *OpenBazaarNode) GetModeratorFee(transactionTotal uint64) (uint64, error) {
	file, err := ioutil.ReadFile(path.Join(n.RepoPath, "root", "moderation"))
	if err != nil {
		return 0, err
	}
	moderator := new(pb.Moderator)
	err = jsonpb.UnmarshalString(string(file), moderator)
	if err != nil {
		return 0, err
	}

	switch moderator.Fee.FeeType {
	case pb.Moderator_Fee_PERCENTAGE:
		return uint64(float64(transactionTotal) * (float64(moderator.Fee.Percentage) / 100)), nil
	case pb.Moderator_Fee_FIXED:
		if strings.ToLower(moderator.Fee.FixedFee.CurrencyCode) == "btc" {
			if moderator.Fee.FixedFee.Amount >= transactionTotal {
				return 0, errors.New("Fixed moderator fee exceeds transaction amount")
			}
			return moderator.Fee.FixedFee.Amount, nil
		} else {
			fee, err := n.getPriceInSatoshi(moderator.Fee.FixedFee.CurrencyCode, moderator.Fee.FixedFee.Amount)
			if err != nil {
				return 0, err
			} else if fee >= transactionTotal {
				return 0, errors.New("Fixed moderator fee exceeds transaction amount")
			}
			return fee, err
		}
	case pb.Moderator_Fee_FIXED_PLUS_PERCENTAGE:
		var fixed uint64
		if strings.ToLower(moderator.Fee.FixedFee.CurrencyCode) == "btc" {
			fixed = moderator.Fee.FixedFee.Amount
		} else {
			fixed, err = n.getPriceInSatoshi(moderator.Fee.FixedFee.CurrencyCode, moderator.Fee.FixedFee.Amount)
			if err != nil {
				return 0, err
			}
		}
		percentage := uint64(float64(transactionTotal) * (float64(moderator.Fee.Percentage) / 100))
		if fixed+percentage >= transactionTotal {
			return 0, errors.New("Fixed moderator fee exceeds transaction amount")
		}
		return fixed + percentage, nil
	default:
		return 0, errors.New("Unrecognized fee type")
	}
}
