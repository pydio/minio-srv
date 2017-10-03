package utils

import (
	"math/big"

	"github.com/pydio/services/common/proto/tree"
)

// TreeNode definition
type TreeNode struct {
	*tree.Node
	MPath MPath
	rat   *Rat
	srat  *Rat
	Level int
}

func NewTreeNode() *TreeNode {
	t := new(TreeNode)
	t.Node = new(tree.Node)
	t.MPath = MPath{}
	t.rat = NewRat()
	t.srat = NewRat()

	return t
}

func (t *TreeNode) Name() string {
	var name string
	t.GetMeta("name", &name)
	return name
}

func (t *TreeNode) Bytes() []byte {
	b, _ := t.rat.GobEncode()

	return b
}

func (t *TreeNode) NV() *big.Int {
	return t.rat.Num()
}

func (t *TreeNode) DV() *big.Int {
	return t.rat.Denom()
}

func (t *TreeNode) SNV() *big.Int {
	return t.srat.Num()
}

func (t *TreeNode) SDV() *big.Int {
	return t.srat.Denom()
}

func (t *TreeNode) SetMPath(mpath ...uint64) {
	t.MPath = MPath(mpath)

	smpath := t.MPath.Sibling()

	t.rat.SetMPath(mpath...)
	t.srat.SetMPath(smpath...)

	t.Level = len(mpath)
}

func (t *TreeNode) SetRat(rat *Rat) {
	mpath := MPath{}

	for rat.Cmp(rat0.Rat) == 1 {
		f, _ := rat.Float64()
		i := int64(f)
		u := uint64(f)

		mpath = append(mpath, u)

		r := NewRat()
		r.SetFrac(big.NewInt(i), int1)

		rat.Sub(rat.Rat, r.Rat)

		if rat.Cmp(rat0.Rat) == 0 {
			break
		}

		rat.Inv(rat.Rat)
		rat.Sub(rat.Rat, rat1.Rat)
		rat.Inv(rat.Rat)
	}

	t.SetMPath(mpath...)
}

func (t *TreeNode) SetBytes(b []byte) {
	r := NewRat()
	r.GobDecode(b)

	t.SetRat(r)
}
