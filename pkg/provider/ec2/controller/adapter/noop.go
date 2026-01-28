package adapter

import (
	"github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	"github.com/kubev2v/forklift/pkg/provider/ec2/controller/builder"
	ec2ensurer "github.com/kubev2v/forklift/pkg/provider/ec2/controller/ensurer"
	"github.com/kubev2v/forklift/pkg/provider/ec2/controller/validator"
)

// Compile-time interface checks. Ensures types implement required base interfaces.
// Catches missing/incorrect methods at compile time, prevents runtime panics.
var _ base.Adapter = &Adapter{}
var _ base.DestinationClient = &DestinationClient{}
var _ base.Builder = &builder.Builder{}
var _ base.Ensurer = &ec2ensurer.Ensurer{}
var _ base.Validator = &validator.Validator{}
