package hare

import (
	"bytes"
	"context"

	"github.com/spacemeshos/go-spacemesh/common/types"
	"github.com/spacemeshos/go-spacemesh/log"
)

type proposalTrackerProvider interface {
	OnProposal(context.Context, *Message)
	OnLateProposal(context.Context, *Message)
	IsConflicting() bool
	ProposedSet() *Set
}

// proposalTracker tracks proposal messages.
type proposalTracker struct {
	logger        log.Log
	malCh         chan<- *types.MalfeasanceGossip
	proposal      *Message
	isConflicting bool
	eTracker      *EligibilityTracker
}

func newProposalTracker(log log.Log, mch chan<- *types.MalfeasanceGossip, et *EligibilityTracker) *proposalTracker {
	return &proposalTracker{
		logger:   log,
		malCh:    mch,
		eTracker: et,
	}
}

// OnProposal tracks the provided proposal message.
// It assumes the proposal message is syntactically valid and that it was received on the proposal round.
func (pt *proposalTracker) OnProposal(ctx context.Context, msg *Message) {
	if pt.proposal == nil { // first leader
		pt.proposal = msg // just update
		return
	}

	// if same sender then we should check for equivocation
	if pt.proposal.SmesherID == msg.SmesherID {
		s := NewSet(msg.Values)
		g := NewSet(pt.proposal.Values)
		if s.Equals(g) {
			return
		}

		// equivocation detected
		pt.logger.WithContext(ctx).With().Warning("equivocation detected in proposal round",
			log.Stringer("smesher", msg.SmesherID),
			log.Stringer("prev", g),
			log.Stringer("curr", s),
		)
		pt.eTracker.Track(msg.SmesherID, msg.Round, msg.Eligibility.Count, false)
		pt.isConflicting = true
		prev := &types.HareProofMsg{
			InnerMsg: types.HareMetadata{
				Layer:   pt.proposal.Layer,
				Round:   pt.proposal.Round,
				MsgHash: types.BytesToHash(pt.proposal.HashBytes()),
			},
			Signature: pt.proposal.Signature,
		}
		this := &types.HareProofMsg{
			InnerMsg: types.HareMetadata{
				Layer:   msg.Layer,
				Round:   msg.Round,
				MsgHash: types.BytesToHash(msg.HashBytes()),
			},
			Signature: msg.Signature,
		}
		if err := reportEquivocation(ctx, msg.SmesherID, prev, this, &msg.Eligibility, pt.malCh); err != nil {
			pt.logger.WithContext(ctx).With().Warning("failed to report equivocation in proposal round",
				log.Stringer("smesher", msg.SmesherID),
				log.Err(err),
			)
		}
		return // process done
	}

	// ignore msgs with higher ranked role proof
	if bytes.Compare(msg.Eligibility.Proof.Bytes(), pt.proposal.Eligibility.Proof.Bytes()) > 0 {
		return
	}

	pt.proposal = msg        // update lower leader msg
	pt.isConflicting = false // assume no conflict
}

// OnLateProposal tracks the given proposal message.
// It assumes the proposal message is syntactically valid and that it was not received on the proposal round (late).
func (pt *proposalTracker) OnLateProposal(ctx context.Context, msg *Message) {
	if pt.proposal == nil {
		return
	}

	// if same sender then we should check for equivocation
	if pt.proposal.SmesherID == msg.SmesherID {
		s := NewSet(msg.Values)
		g := NewSet(pt.proposal.Values)
		if !s.Equals(g) { // equivocation detected
			pt.logger.WithContext(ctx).With().Warning("equivocation detected in proposal round - late",
				log.Stringer("smesher", msg.SmesherID),
				log.Stringer("prev", g),
				log.Stringer("curr", s),
			)
			pt.eTracker.Track(msg.SmesherID, msg.Round, msg.Eligibility.Count, false)
			pt.isConflicting = true
			prev := &types.HareProofMsg{
				InnerMsg: types.HareMetadata{
					Layer:   pt.proposal.Layer,
					Round:   pt.proposal.Round,
					MsgHash: types.BytesToHash(pt.proposal.HashBytes()),
				},
				Signature: pt.proposal.Signature,
			}
			this := &types.HareProofMsg{
				InnerMsg: types.HareMetadata{
					Layer:   msg.Layer,
					Round:   msg.Round,
					MsgHash: types.BytesToHash(msg.HashBytes()),
				},
				Signature: msg.Signature,
			}
			if err := reportEquivocation(ctx, msg.SmesherID, prev, this, &msg.Eligibility, pt.malCh); err != nil {
				pt.logger.WithContext(ctx).With().Warning("failed to report equivocation in proposal round - late",
					log.Stringer("smesher", msg.SmesherID),
					log.Err(err),
				)
			}
			pt.isConflicting = true
		}
	}

	// not equal check rank
	// lower ranked proposal on late proposal is a conflict
	if bytes.Compare(msg.Eligibility.Proof.Bytes(), pt.proposal.Eligibility.Proof.Bytes()) < 0 {
		pt.logger.WithContext(ctx).With().Warning("late lower rank detected",
			log.String("id_malicious", msg.SmesherID.String()),
		)
		pt.isConflicting = true
	}
}

// IsConflicting returns true if there was a conflict, false otherwise.
func (pt *proposalTracker) IsConflicting() bool {
	return pt.isConflicting
}

// ProposedSet returns the proposed set if there is a valid proposal, nil otherwise.
func (pt *proposalTracker) ProposedSet() *Set {
	if pt.proposal == nil {
		return nil
	}

	if pt.isConflicting {
		return nil
	}

	return NewSet(pt.proposal.Values)
}
