package localstore

import (
	"errors"

	"github.com/ethsana/sana/pkg/shed"
	"github.com/ethsana/sana/pkg/storage"
	"github.com/ethsana/sana/pkg/swarm"
	"github.com/syndtr/goleveldb/leveldb"
)

// pinCounter returns the pin counter for a given swarm address, provided that the
// address has been pinned.
func (db *DB) pinCounter(address swarm.Address) (uint64, error) {
	out, err := db.pinIndex.Get(shed.Item{
		Address: address.Bytes(),
	})

	if err != nil {
		if errors.Is(err, leveldb.ErrNotFound) {
			return 0, storage.ErrNotFound
		}
		return 0, err
	}
	return out.PinCounter, nil
}
