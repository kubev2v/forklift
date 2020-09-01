package vsphere

import (
	"errors"
	liberr "github.com/konveyor/controller/pkg/error"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	libref "github.com/konveyor/controller/pkg/ref"
)

//
// Kinds
var (
	FolderKind     = libref.ToKind(Folder{})
	DatacenterKind = libref.ToKind(Datacenter{})
	ClusterKind    = libref.ToKind(Cluster{})
	HostKind       = libref.ToKind(Host{})
	NetKind        = libref.ToKind(Network{})
	DsKind         = libref.ToKind(Datastore{})
	VmKind         = libref.ToKind(VM{})
)

//
// Errors
var (
	// Invalid ref.
	InvalidRefErr = liberr.New("invalid ref")
	// Invalid kind in ref encountered during tree traversal.
	InvalidKindErr = liberr.New("invalid kind")
)

//
// Tree.
type Tree struct {
	// DB connection.
	DB libmodel.DB
	// Tree root.
	Root Model
	// Leaf kind.
	Leaf string
	// Flatten the tree (root & leafs).
	Flatten bool
	// Depth limit (0=unlimited).
	Depth int
}

//
// Build the tree.
func (r *Tree) Build() (*TreeNode, error) {
	kind := libref.ToKind(r.Root)
	root := &TreeNode{
		Kind:  kind,
		Model: r.Root,
	}
	node := root
	depth := 0
	var walk func(Model, bool) error
	walk = func(model Model, asChild bool) error {
		kind := libref.ToKind(model)
		if asChild {
			child := &TreeNode{
				Parent: node,
				Kind:   kind,
				Model:  model,
			}
			if !r.Flatten {
				depth++
				defer func() {
					depth--
				}()
			}
			if r.Depth > 0 && depth > r.Depth {
				return nil
			}
			if !r.Flatten || kind == r.Leaf {
				node.Children = append(node.Children, child)
			}
			if !r.Flatten {
				node = child
				defer func() {
					node = node.Parent
				}()
			}
		}
		switch kind {
		case FolderKind:
			folder := model.(*Folder)
			refList := RefList{}
			refList.With(folder.Children)
		next:
			for _, ref := range refList {
				switch r.Leaf {
				case FolderKind:
					if ref.Kind != r.Leaf {
						continue next
					}
				}
				m, err := r.getRef(ref)
				if err != nil {
					if errors.Is(err, InvalidRefErr) {
						continue
					}
					return liberr.Wrap(err)
				}
				err = walk(m, true)
				if err != nil {
					return liberr.Wrap(err)
				}
			}
		case DatacenterKind:
			var ref Ref
			dc := model.(*Datacenter)
			switch r.Leaf {
			case ClusterKind, HostKind:
				ref.With(dc.Clusters)
			case VmKind:
				ref.With(dc.Vms)
			case NetKind:
				ref.With(dc.Networks)
			case DsKind:
				ref.With(dc.Datastores)
			case DatacenterKind:
				// Leaf
			default:
				return InvalidKindErr
			}
			m, err := r.getRef(ref)
			if err != nil {
				if errors.Is(err, InvalidRefErr) {
					return nil
				}
				return liberr.Wrap(err)
			}
			err = walk(m, true)
			if err != nil {
				return liberr.Wrap(err)
			}
		case ClusterKind:
			refList := RefList{}
			cluster := model.(*Cluster)
			switch r.Leaf {
			case HostKind, VmKind:
				refList.With(cluster.Hosts)
			case NetKind:
				refList.With(cluster.Networks)
			case DsKind:
				refList.With(cluster.Datastores)
			case ClusterKind:
				// Leaf
			default:
				return InvalidKindErr
			}
			for _, ref := range refList {
				m, err := r.getRef(ref)
				if err != nil {
					if errors.Is(err, InvalidRefErr) {
						return nil
					}
					return liberr.Wrap(err)
				}
				err = walk(m, true)
				if err != nil {
					return liberr.Wrap(err)
				}
			}
		case HostKind:
			refList := RefList{}
			host := model.(*Host)
			switch r.Leaf {
			case VmKind:
				refList.With(host.Vms)
			case NetKind:
				refList.With(host.Networks)
			case DsKind:
				refList.With(host.Datastores)
			case HostKind:
				// Leaf
			default:
				return InvalidKindErr
			}
			for _, ref := range refList {
				m, err := r.getRef(ref)
				if err != nil {
					if errors.Is(err, InvalidRefErr) {
						return nil
					}
					return liberr.Wrap(err)
				}
				err = walk(m, true)
				if err != nil {
					return liberr.Wrap(err)
				}
			}
		case VmKind:
			// Leaf
		case NetKind:
			// Leaf
		case DsKind:
			// Leaf
		default:
			return InvalidKindErr
		}

		return nil
	}
	err := walk(r.Root, false)
	if err != nil {
		return nil, liberr.Wrap(err)
	}

	return root, nil
}

//
// Get referenced model.
func (r *Tree) getRef(ref Ref) (model Model, err error) {
	base := Base{
		ID: ref.ID,
	}
	switch ref.Kind {
	case FolderKind:
		model = &Folder{Base: base}
	case DatacenterKind:
		model = &Datacenter{Base: base}
	case ClusterKind:
		model = &Cluster{Base: base}
	case HostKind:
		model = &Host{Base: base}
	case VmKind:
		model = &VM{Base: base}
	case NetKind:
		model = &Network{Base: base}
	case DsKind:
		model = &Datastore{Base: base}
	default:
		err = InvalidRefErr
	}
	if model != nil {
		err = r.DB.Get(model)
	}

	return
}

//
// Tree node.
type TreeNode struct {
	// Parent node.
	Parent *TreeNode
	// Kind of model.
	Kind string
	// Model.
	Model Model
	// Child nodes.
	Children []*TreeNode
}
