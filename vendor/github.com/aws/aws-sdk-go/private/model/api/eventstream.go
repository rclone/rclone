// +build codegen

package api

func (a *API) suppressEventStreams() {
	const eventStreamMemberName = "EventStream"

	for name, op := range a.Operations {
		outbound := hasEventStream(op.InputRef.Shape)
		inbound := hasEventStream(op.OutputRef.Shape)

		if !(outbound || inbound) {
			continue
		}

		a.removeOperation(name)
	}
}

func hasEventStream(topShape *Shape) bool {
	for _, ref := range topShape.MemberRefs {
		if ref.Shape.IsEventStream {
			return true
		}
	}

	return false
}
