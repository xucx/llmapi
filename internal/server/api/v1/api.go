package v1

import (
	context "context"
	"fmt"

	"github.com/xucx/llmapi"

	apiv1 "github.com/xucx/llmapi/api/v1"
	"github.com/xucx/llmapi/internal/server/api/v1/util"
	v1util "github.com/xucx/llmapi/internal/server/api/v1/util"
	"github.com/xucx/llmapi/log"
	"github.com/xucx/llmapi/types"

	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

var (
	GrpcArgumentError = status.Errorf(codes.InvalidArgument, "Invalid Argument")
	GrpcInternalError = status.Errorf(codes.Internal, "Internal Server Error")
)

type ApiService struct {
	apiv1.UnimplementedApiServiceServer
	models *llmapi.Models
}

func NewApiService(models *llmapi.Models) *ApiService {
	return &ApiService{
		models: models,
	}
}

func (s *ApiService) Chat(ctx context.Context, req *apiv1.ChatRequest) (*apiv1.ChatResponse, error) {
	if req.ChatParams == nil {
		return nil, GrpcArgumentError
	}

	messages := []*types.Message{}
	for _, m := range req.ChatParams.Messages {
		msg, err := v1util.ToMessage(m)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}

	options, err := v1util.ChatParamsToOptions(req.ChatParams)
	if err != nil {
		return nil, GrpcArgumentError
	}

	completion, err := s.models.Generate(ctx, req.ChatParams.Model, messages, types.ChatWithOptions(options))
	if err != nil {
		log.Errorw("llm chat fail", "model", req.ChatParams.Model, "error", err)
		return nil, GrpcInternalError
	}

	retCompletion, err := v1util.FromChatCompletion(completion)
	if err != nil {
		return nil, GrpcInternalError
	}

	return &apiv1.ChatResponse{
		ChatCompletion: retCompletion,
	}, nil
}

func (s *ApiService) ChatStream(req *apiv1.ChatStreamRequest, stream apiv1.ApiService_ChatStreamServer) error {

	if req.ChatParams == nil {
		return GrpcArgumentError
	}

	messages := []*types.Message{}
	for _, m := range req.ChatParams.Messages {
		msg, err := v1util.ToMessage(m)
		if err != nil {
			return err
		}
		messages = append(messages, msg)
	}

	options, err := v1util.ChatParamsToOptions(req.ChatParams)
	if err != nil {
		return GrpcArgumentError
	}

	completion, err := s.models.Generate(stream.Context(), req.ChatParams.Model, messages, types.ChatWithOptions(options),
		types.ChatWithStreamingFunc(func(ctx context.Context, c *types.Completion) error {
			log.Debugw("api recv chunck completeion", "completion", c)

			deltaCompletion, err := util.FromChatCompletion(c)
			if err != nil {
				return err
			}

			log.Debugw("return stream delta completion", "completion", deltaCompletion)
			return stream.Send(&apiv1.ChatStreamResponse{
				ChatCompletion: deltaCompletion,
			})
		}),
	)

	if err != nil {
		log.Errorw("llm chat fail", "model", req.ChatParams.Model, "error", err)
		return GrpcInternalError
	}

	log.Debugw("api recv completeion", "completion", completion)

	fullCompletion, err := util.FromChatCompletion(completion)
	if err != nil {
		return GrpcInternalError
	}

	log.Debugw("return stream completion", "completion", fullCompletion)

	return stream.Send(&apiv1.ChatStreamResponse{
		ChatCompletion: fullCompletion,
	})
}

func (s *ApiService) ChatRealtime(stream apiv1.ApiService_ChatRealtimeServer) error {
	ctx, cancel := context.WithCancel(stream.Context())
	defer cancel()

	session, err := s.initRealtime(stream)
	if err != nil {
		return GrpcInternalError
	}

	go func() {
		defer cancel()

		for {
			req, err := stream.Recv()
			if err != nil {
				return
			}

			message, err := v1util.ToMessage(req.Message)
			if err != nil {
				return
			}

			if err := session.Send(ctx, message); err != nil {
				return
			}
		}
	}()

	for {
		rsp, err := session.Recv(ctx)
		if err != nil {
			break
		}

		completion, err := v1util.FromChatCompletion(rsp)
		if err != nil {
			return err
		}

		if err := stream.Send(&apiv1.ChatRealtimeResponse{
			ChatCompletion: completion,
		}); err != nil {
			return err
		}
	}

	return nil
}

func (s *ApiService) initRealtime(stream apiv1.ApiService_ChatRealtimeServer) (types.RealTimeSession, error) {
	ctx := stream.Context()

	req, err := stream.Recv()
	if err != nil {
		return nil, err
	}

	if req.Init == nil {
		return nil, fmt.Errorf("no init message")
	}

	messages := []*types.Message{}
	for _, m := range req.Init.ChatParams.Messages {
		msg, err := v1util.ToMessage(m)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}

	options := []types.RealTimeOption{}
	if req.Init.ChatParams.Model != "" {
		options = append(options, types.RealTimeWithModel(req.Init.ChatParams.Model))
	}
	if req.Init.ChatParams.Instructions != "" {
		options = append(options, types.RealTimeWithInstructions(req.Init.ChatParams.Instructions))
	}

	if req.Init.ChatParams.Tools != nil {
		tools, err := v1util.ToChatTools(req.Init.ChatParams.Tools)
		if err != nil {
			return nil, err
		}
		options = append(options, types.RealTimeWithTools(tools))
	}
	if req.Init.ChatParams.Voice != "" {
		voice, err := v1util.ToChatVoice(req.Init.ChatParams.Voice)
		if err != nil {
			return nil, err
		}
		options = append(options, types.RealTimeWithAudioVoice(voice))
	}

	session, err := s.models.Realtime(ctx, req.Init.ChatParams.Model, messages, options...)
	if err != nil {
		return nil, err
	}

	return session, nil
}
