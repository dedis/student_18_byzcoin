package collection

import "testing"
import "encoding/binary"

func TestProofDumpNode(test *testing.T) {
	stake64 := Stake64{}
	data := Data{}

	collection := New(stake64, data)
	collection.Add([]byte("mykey"), uint64(66), []byte("myvalue"))

	rootdump := dumpnode(collection.root)

	if rootdump.Label != collection.root.label {
		test.Error("[proof.go]", "[dumpnode]", "dumpnode() sets wrong label on dump of internal node.")
	}

	if len(rootdump.Key) != 0 {
		test.Error("[proof.go]", "[dumpnode]", "dumpnode() sets key on internal node.")
	}

	if len(rootdump.Values) != 2 {
		test.Error("[proof.go]", "[dumpnode]", "dumpnode() sets the wrong number of values on internal node.")
	}

	if !equal(rootdump.Values[0], collection.root.values[0]) || !equal(rootdump.Values[1], collection.root.values[1]) {
		test.Error("[proof.go]", "[dumpnode]", "dumpnode() sets the wrong values on internal node.")
	}

	if (rootdump.Children.Left != collection.root.children.left.label) || (rootdump.Children.Right != collection.root.children.right.label) {
		test.Error("[proof.go]", "[dumpnode]", "dumpnode() sets the wrong children labels on internal node.")
	}

	var leaf *node

	if collection.root.children.left.placeholder() {
		leaf = collection.root.children.right
	} else {
		leaf = collection.root.children.left
	}

	leafdump := dumpnode(leaf)

	if leafdump.Label != leaf.label {
		test.Error("[proof.go]", "[dumpnode]", "dumpnode() sets wrong label on dump of leaf.")
	}

	if !equal(leafdump.Key, leaf.key) {
		test.Error("[proof.go]", "[dumpnode]", "dumpnode() sets wrong key on leaf.")
	}

	if len(leafdump.Values) != 2 {
		test.Error("[proof.go]", "[dumpnode]", "dumpnode() sets the wrong number of values on leaf.")
	}

	if !equal(leafdump.Values[0], leaf.values[0]) || !equal(leafdump.Values[1], leaf.values[1]) {
		test.Error("[proof.go]", "[dumpnode]", "dumpnode() sets the wrong values on leaf.")
	}

	var empty [hashSize]byte

	if (leafdump.Children.Left != empty) || (leafdump.Children.Right != empty) {
		test.Error("[proof.go]", "[dumpnode]", "dumpnode() sets non-null children labels on leaf.")
	}
}

func TestProofDumpGetters(test *testing.T) {
	stake64 := Stake64{}
	data := Data{}

	collection := New(stake64, data)
	collection.Add([]byte("mykey"), uint64(66), []byte("myvalue"))

	rootdump := dumpnode(collection.root)

	var leaf *node

	if collection.root.children.left.placeholder() {
		leaf = collection.root.children.right
	} else {
		leaf = collection.root.children.left
	}

	leafdump := dumpnode(leaf)

	if rootdump.leaf() {
		test.Error("[proof.go]", "[dumpgetters]", "leaf() returns true on internal node.")
	}

	if !(leafdump.leaf()) {
		test.Error("[proof.go]", "[dumpgetters]", "leaf() returns false on leaf node.")
	}
}

func TestProofDumpConsistent(test *testing.T) {
	stake64 := Stake64{}
	data := Data{}

	collection := New(stake64, data)
	collection.Add([]byte("mykey"), uint64(66), []byte("myvalue"))

	rootdump := dumpnode(collection.root)

	var leaf *node

	if collection.root.children.left.placeholder() {
		leaf = collection.root.children.right
	} else {
		leaf = collection.root.children.left
	}

	leafdump := dumpnode(leaf)

	if !(rootdump.consistent()) {
		test.Error("[proof.go]", "[consistent]", "consistent() returns false on valid internal node.")
	}

	rootdump.Label[0]++

	if rootdump.consistent() {
		test.Error("[proof.go]", "[consistent]", "consistent() returns true on invalid internal node.")
	}

	if !(leafdump.consistent()) {
		test.Error("[proof.go]", "[consistent]", "consistent() returns false on valid leaf.")
	}

	leafdump.Label[0]++

	if leafdump.consistent() {
		test.Error("[proof.go]", "[consistent]", "consistent() returns true on invalid leaf.")
	}
}

func TestProofDumpTo(test *testing.T) {
	ctx := testctx("[proof.go]", test)

	stake64 := Stake64{}
	data := Data{}

	collection := New(stake64, data)
	collection.Add([]byte("mykey"), uint64(66), []byte("myvalue"))

	rootdump := dumpnode(collection.root)
	leftdump := dumpnode(collection.root.children.left)
	rightdump := dumpnode(collection.root.children.right)

	unknown := New(stake64, data)
	unknown.scope.None()

	unknown.Begin()
	unknown.Add([]byte("mykey"), uint64(66), []byte("myvalue"))
	unknown.End()

	rootdump.to(unknown.root)

	if !(unknown.root.known) {
		test.Error("[proof.go]", "[to]", "Method to() does not set known to true.")
	}

	if (unknown.root.children.left == nil) || (unknown.root.children.right == nil) {
		test.Error("[proof.go]", "[to]", "Method to() does not branch internal nodes.")
	}

	leftdump.to(unknown.root.children.left)
	rightdump.to(unknown.root.children.right)

	if !(unknown.root.children.left.known) {
		test.Error("[proof.go]", "[to]", "Method to() does not set known to true.")
	}

	if !(unknown.root.children.right.known) {
		test.Error("[proof.go]", "[to]", "Method to() does not set known to true.")
	}

	if unknown.root.label != collection.root.label {
		test.Error("[proof.go]", "[to]", "Method to() corrupts the label of an internal node.")
	}

	unknown.fix()

	if unknown.root.label != collection.root.label {
		test.Error("[proof.go]", "[to]", "Fixing a collection expanded from dumps has a non-null effect on the root label.")
	}

	ctx.verify.tree("[to]", &unknown)

	leftdump.to(unknown.root.children.right)
	unknown.fix()
	ctx.verify.tree("[to]", &unknown)

	if unknown.root.label != collection.root.label {
		test.Error("[proof.go]", "[to]", "Method to() has non-null effect when used on node with non-matching label.")
	}
}

func TestProofGetters(test *testing.T) {
	proof := Proof{}
	proof.key = []byte("mykey")

	if !equal(proof.Key(), []byte("mykey")) {
		test.Error("[proof.go]", "[proofgetters]", "Key() returns wrong key.")
	}
}

func TestProofMatchValues(test *testing.T) {
	collision := func(key []byte, bits int) []byte {
		target := hash(key)
		sample := make([]byte, 8)

		for index := 0; ; index++ {
			binary.BigEndian.PutUint64(sample, uint64(index))
			hash := hash(sample)
			if match(hash[:], target[:], bits) && !match(hash[:], target[:], bits+1) {
				return sample
			}
		}
	}

	stake64 := Stake64{}
	data := Data{}

	firstkey := []byte("mykey")
	secondkey := collision(firstkey, 5)

	collection := New(stake64, data)
	collection.Add(firstkey, uint64(66), []byte("firstvalue"))
	collection.Add(secondkey, uint64(99), []byte("secondvalue"))

	proof := Proof{}
	proof.collection = &collection
	proof.key = firstkey
	proof.root = dumpnode(collection.root)

	path := hash(firstkey)
	cursor := collection.root

	for depth := 0; depth < 6; depth++ {
		proof.steps = append(proof.steps, step{dumpnode(cursor.children.left), dumpnode(cursor.children.right)})

		if bit(path[:], depth) {
			cursor = cursor.children.right
		} else {
			cursor = cursor.children.left
		}
	}

	if !(proof.Match()) {
		test.Error("[proof.go]", "[match]", "Proof Match() returns false on matching key.")
	}

	firstvalues, err := proof.Values()

	if err != nil {
		test.Error("[proof.go]", "[values]", "Proof Values() returns error on matching key.")
	}

	if len(firstvalues) != 2 {
		test.Error("[proof.go]", "[values]", "Proof Values() returns wrong number of values.")
	}

	if (firstvalues[0].(uint64) != 66) || !equal(firstvalues[1].([]byte), []byte("firstvalue")) {
		test.Error("[proof.go]", "[values]", "Proof Values() returns wrong values.")
	}

	proof.key = secondkey

	if !(proof.Match()) {
		test.Error("[proof.go]", "[match]", "Proof Match() returns false on matching key.")
	}

	secondvalues, err := proof.Values()

	if err != nil {
		test.Error("[proof.go]", "[values]", "Proof Values() returns error on matching key.")
	}

	if len(secondvalues) != 2 {
		test.Error("[proof.go]", "[values]", "Proof Values() returns wrong number of values.")
	}

	if (secondvalues[0].(uint64) != 99) || !equal(secondvalues[1].([]byte), []byte("secondvalue")) {
		test.Error("[proof.go]", "[values]", "Proof Values() returns wrong values.")
	}

	proof.key = []byte("wrongkey")

	if proof.Match() {
		test.Error("[proof.go]", "[match]", "Proof Match() returns true on non-matching key.")
	}

	_, err = proof.Values()

	if err == nil {
		test.Error("[proof.go]", "[values]", "Proof Values() does not yield an error on non-matching key.")
	}

	proof.key = firstkey

	proof.steps[5].Left.Values[0] = make([]byte, 7)
	proof.steps[5].Right.Values[0] = make([]byte, 7)

	_, err = proof.Values()

	if err == nil {
		test.Error("[proof.go]", "[values]", "Proof Values() does not yield an error on a record with ill-formed values.")
	}

	proof.steps[5].Left.Values = [][]byte{make([]byte, 8)}
	proof.steps[5].Left.Values = [][]byte{make([]byte, 8)}

	_, err = proof.Values()

	if err == nil {
		test.Error("[proof.go]", "[values]", "Proof Values() does not yield an error on a record with wrong number of values.")
	}

	proof.steps = []step{}

	if proof.Match() {
		test.Error("[proof.go]", "[match]", "Proof Match() returns true on a proof with no steps.")
	}

	_, err = proof.Values()

	if err == nil {
		test.Error("[proof.go]", "[values]", "Proof Values() does not yield an error on a proof with no steps.")
	}
}

func TestProofConsistent(test *testing.T) {
	stake64 := Stake64{}
	collection := New(stake64)

	for index := 0; index < 512; index++ {
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, uint64(index))

		collection.Add(key, uint64(index))
	}

	key := make([]byte, 8)
	proof, _ := collection.Get(key).Proof()

	if !(proof.consistent()) {
		test.Error("[proof.go]", "[consistent]", "Proof produced by collection is not consistent.")
	}

	proof.root.Label[0]++
	if proof.consistent() {
		test.Error("[proof.go]", "[consistent]", "Proof is still consistent after altering label of root node.")
	}
	proof.root.Label[0]--

	proof.root.Values[0][0]++
	if proof.consistent() {
		test.Error("[proof.go]", "[consistent]", "Proof is still consistent after altering values of root node.")
	}
	proof.root.Values[0][0]--

	proof.root.Children.Left[0]++
	if proof.consistent() {
		test.Error("[proof.go]", "[consistent]", "Proof is still consistent after altering label of left child of root node.")
	}
	proof.root.Children.Left[0]--

	proof.root.Children.Right[0]++
	if proof.consistent() {
		test.Error("[proof.go]", "[consistent]", "Proof is still consistent after altering label of root node.")
	}
	proof.root.Children.Right[0]--

	stepsbackup := proof.steps
	proof.steps = []step{}
	if proof.consistent() {
		test.Error("[proof.go]", "[consistent]", "Proof with no steps is still consisetent.")
	}
	proof.steps = stepsbackup

	for index := 0; index < len(proof.steps); index++ {
		step := &(proof.steps[index])

		step.Left.Label[0]++
		if proof.consistent() {
			test.Error("[proof.go]", "[consistent]", "Proof is still consistent after altering label of one of left steps.")
		}
		step.Left.Label[0]--

		step.Right.Label[0]++
		if proof.consistent() {
			test.Error("[proof.go]", "[consistent]", "Proof is still consistent after altering label of one of right steps.")
		}
		step.Right.Label[0]--

		step.Left.Values[0][0]++
		if proof.consistent() {
			test.Error("[proof.go]", "[consistent]", "Proof is still consistent after altering value of one of left steps.")
		}
		step.Left.Values[0][0]--

		step.Right.Values[0][0]++
		if proof.consistent() {
			test.Error("[proof.go]", "[consistent]", "Proof is still consistent after altering value of one of right steps.")
		}
		step.Right.Values[0][0]--

		if step.Left.leaf() {
			placeholder := (len(step.Left.Key) == 0)
			if !placeholder {
				step.Left.Key[0]++
			} else {
				step.Left.Key = []byte("x")
			}

			if proof.consistent() {
				test.Error("[proof.go]", "[consistent]", "Proof is still consistent after altering key of one of left leaf steps.")
			}

			if !placeholder {
				step.Left.Key[0]--
			} else {
				step.Left.Key = []byte{}
			}
		} else {
			step.Left.Children.Left[0]++
			if proof.consistent() {
				test.Error("[proof.go]", "[consistent]", "Proof is still consistent after altering left child of one of left internal node steps.")
			}
			step.Left.Children.Left[0]--

			step.Left.Children.Right[0]++
			if proof.consistent() {
				test.Error("[proof.go]", "[consistent]", "Proof is still consistent after altering right child of one of left internal node steps.")
			}
			step.Left.Children.Right[0]--
		}

		if step.Right.leaf() {
			placeholder := (len(step.Right.Key) == 0)
			if !placeholder {
				step.Right.Key[0]++
			} else {
				step.Right.Key = []byte("x")
			}

			if proof.consistent() {
				test.Error("[proof.go]", "[consistent]", "Proof is still consistent after altering key of one of right leaf steps.")
			}

			if !placeholder {
				step.Right.Key[0]--
			} else {
				step.Right.Key = []byte{}
			}
		} else {
			step.Right.Children.Left[0]++
			if proof.consistent() {
				test.Error("[proof.go]", "[consistent]", "Proof is still consistent after altering left child of one of right internal node steps.")
			}
			step.Right.Children.Left[0]--

			step.Right.Children.Right[0]++
			if proof.consistent() {
				test.Error("[proof.go]", "[consistent]", "Proof is still consistent after altering right child of one of right internal node steps.")
			}
			step.Right.Children.Right[0]--
		}
	}

	if !(proof.consistent()) {
		test.Error("[proof.go]", "[consistent]", "Proof is not consistent after reversing all the updates, check test.")
	}
}

func TestProofSerialization(test *testing.T) {
	stake64 := Stake64{}
	data := Data{}

	collection := New(stake64, data)

	for index := 0; index < 512; index++ {
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, uint64(index))

		collection.Add(key, uint64(index), key)
	}

	for index := 0; index < 512; index++ {
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, uint64(index))

		proof, _ := collection.Get(key).Proof()
		buffer := collection.Serialize(proof)

		otherproof, error := collection.Deserialize(buffer)

		if error != nil {
			test.Error("[proof.go]", "[serialization]", "Serialize() / Deserialize() yields an error on a valid proof.")
		}

		if otherproof.collection != &collection {
			test.Error("[proof.go]", "[serialization]", "Deserialize() does not properly set the collection pointer.")
		}

		if !(collection.Verify(otherproof)) {
			test.Error("[proof.go]", "[serialization]", "Serialize() / Deserialize() yields an invalid proof on a valid proof.")
		}
	}

	_, error := collection.Deserialize([]byte("definitelynotaproof"))

	if error == nil {
		test.Error("[proof.go]", "[serialization]", "Deserialize() does not yield an error when provided with an invalid byte slice.")
	}
}
