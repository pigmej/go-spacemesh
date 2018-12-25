package hare

import (
	"github.com/gogo/protobuf/proto"
	"github.com/spacemeshos/go-spacemesh/hare/pb"
	"github.com/spacemeshos/go-spacemesh/p2p/service"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

var instanceId1 = &InstanceId{Bytes32{1}}
var instanceId2 = &InstanceId{Bytes32{2}}
var instanceId3 = &InstanceId{Bytes32{3}}

func createMessage(t *testing.T, instanceId Byteable) []byte {
	hareMsg := &pb.HareMessage{}
	hareMsg.Message = &pb.InnerMessage{InstanceId: instanceId.Bytes()}
	serMsg, err := proto.Marshal(hareMsg)

	if err != nil {
		assert.Fail(t, "Failed to marshal data")
	}

	return serMsg
}

type MockInboxer struct {
	inbox chan *pb.HareMessage
}

func (inboxer *MockInboxer) createInbox(size uint32) chan *pb.HareMessage {
	inboxer.inbox = make(chan *pb.HareMessage, size)
	return inboxer.inbox
}

func TestBroker_Start(t *testing.T) {
	sim := service.NewSimulator()
	n1 := sim.NewNode()
	broker := NewBroker(n1)

	err := broker.Start()
	assert.Equal(t, nil, err)

	err = broker.Start()
	assert.Equal(t, "instance already started", err.Error())
}

// test that a message to a specific set Id is delivered by the broker
func TestBroker_Received(t *testing.T) {
	sim := service.NewSimulator()
	n1 := sim.NewNode()
	n2 := sim.NewNode()

	broker := NewBroker(n1)
	broker.Start()

	inboxer := &MockInboxer{}
	broker.Register(instanceId1, inboxer)

	serMsg := createMessage(t, instanceId1)
	n2.Broadcast(ProtoName, serMsg)

	recv := <-inboxer.inbox

	assert.True(t, recv.Message.InstanceId[0] == instanceId1.Bytes()[0])
}

// test that aborting the broker aborts
func TestBroker_Abort(t *testing.T) {
	sim := service.NewSimulator()
	n1 := sim.NewNode()

	broker := NewBroker(n1)
	broker.Start()

	timer := time.NewTimer(3 * time.Second)

	go broker.Close()

	select {
	case <-broker.CloseChannel():
		assert.True(t, true)
	case <-timer.C:
		assert.Fail(t, "timeout")
	}
}

func sendMessages(t *testing.T, instanceId *InstanceId, n *service.Node, count int) {
	for i := 0; i < count; i++ {
		n.Broadcast(ProtoName, createMessage(t, instanceId))
	}
}

func waitForMessages(t *testing.T, inbox chan *pb.HareMessage, instanceId *InstanceId, msgCount int) {
	for i := 0; i < msgCount; i++ {
		x := <-inbox
		assert.True(t, x.Message.InstanceId[0] == instanceId.Bytes()[0])
	}
}

// test flow for multiple set id
func TestBroker_MultipleInstanceIds(t *testing.T) {
	sim := service.NewSimulator()
	n1 := sim.NewNode()
	n2 := sim.NewNode()
	const msgCount = 100

	broker := NewBroker(n1)
	broker.Start()

	inboxer1 := &MockInboxer{}
	inboxer2 := &MockInboxer{}
	inboxer3 := &MockInboxer{}
	broker.Register(instanceId1, inboxer1)
	broker.Register(instanceId2, inboxer2)
	broker.Register(instanceId3, inboxer3)

	inbox1 := inboxer1.inbox
	inbox2 := inboxer2.inbox
	inbox3 := inboxer3.inbox

	go sendMessages(t, instanceId1, n2, msgCount)
	go sendMessages(t, instanceId2, n2, msgCount)
	go sendMessages(t, instanceId3, n2, msgCount)

	waitForMessages(t, inbox1, instanceId1, msgCount)
	waitForMessages(t, inbox2, instanceId2, msgCount)
	waitForMessages(t, inbox3, instanceId3, msgCount)

	assert.True(t, true)
}

func TestBroker_RegisterUnregister(t *testing.T) {
	sim := service.NewSimulator()
	n1 := sim.NewNode()
	broker := NewBroker(n1)
	broker.Start()
	inbox := &MockInboxer{}
	broker.Register(instanceId1, inbox)
	assert.Equal(t, 1, len(broker.outbox))
	broker.Unregister(instanceId1)
	assert.Equal(t, 0, len(broker.outbox))
}
