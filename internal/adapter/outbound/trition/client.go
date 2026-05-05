package triton

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mohammadpnp/content-moderator/api/triton"
	"github.com/mohammadpnp/content-moderator/internal/domain/entity"
	"github.com/mohammadpnp/content-moderator/internal/domain/port/outbound"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	textModelName  = "text_moderation"
	imageModelName = "image_moderation"
	maxRetries     = 2
)

type TritonClient struct {
	conn   *grpc.ClientConn
	client triton.GRPCInferenceServiceClient
}

func NewTritonClient(addr string) (*TritonClient, error) {
	conn, err := grpc.Dial(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithTimeout(10*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to dial Triton: %w", err)
	}
	return &TritonClient{
		conn:   conn,
		client: triton.NewGRPCInferenceServiceClient(conn),
	}, nil
}

func (c *TritonClient) Close() error {
	return c.conn.Close()
}

func (c *TritonClient) ModerateText(ctx context.Context, text string) (*entity.ModerationResult, error) {
	input := &triton.ModelInferRequest_InferInputTensor{
		Name:     "text_input",
		Datatype: "BYTES",
		Shape:    []int64{1},
		Contents: &triton.InferTensorContents{
			BytesContents: [][]byte{[]byte(text)},
		},
	}

	outputs := []*triton.ModelInferRequest_InferRequestedOutputTensor{
		{Name: "probabilities"},
		{Name: "categories"},
	}

	req := &triton.ModelInferRequest{
		ModelName:    textModelName,
		Inputs:       []*triton.ModelInferRequest_InferInputTensor{input},
		Outputs:      outputs,
		ModelVersion: "",
	}

	var resp *triton.ModelInferResponse
	var err error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(100 * time.Millisecond):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		resp, err = c.client.ModelInfer(ctx, req)
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, fmt.Errorf("Triton ModelInfer error: %w", err)
	}

	result, err := parseTextModerationResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}
	return result, nil
}

func (c *TritonClient) ModerateImage(ctx context.Context, imageURL string) (*entity.ModerationResult, error) {
	input := &triton.ModelInferRequest_InferInputTensor{
		Name:     "image_input",
		Datatype: "BYTES",
		Shape:    []int64{1},
		Contents: &triton.InferTensorContents{
			BytesContents: [][]byte{[]byte(imageURL)},
		},
	}

	outputs := []*triton.ModelInferRequest_InferRequestedOutputTensor{
		{Name: "probabilities"},
		{Name: "categories"},
	}

	req := &triton.ModelInferRequest{
		ModelName:    imageModelName,
		Inputs:       []*triton.ModelInferRequest_InferInputTensor{input},
		Outputs:      outputs,
		ModelVersion: "",
	}

	resp, err := c.client.ModelInfer(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("Triton ModelInfer error: %w", err)
	}

	result, err := parseImageModerationResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}
	return result, nil
}

func (c *TritonClient) IsHealthy(ctx context.Context) bool {
	_, err := c.client.ServerLive(ctx, &triton.ServerLiveRequest{})
	return err == nil
}

func parseTextModerationResponse(resp *triton.ModelInferResponse) (*entity.ModerationResult, error) {
	var score float64
	var categories []entity.ModerationCategory

	for _, out := range resp.Outputs {
		switch out.Name {
		case "probabilities":
			if len(out.Contents.Fp32Contents) >= 2 {
				score = float64(out.Contents.Fp32Contents[1])
			}
		case "categories":
			if len(out.Contents.BytesContents) > 0 {
				catsStr := string(out.Contents.BytesContents[0])
				for _, cat := range strings.Split(catsStr, ",") {
					categories = append(categories, entity.ModerationCategory(strings.TrimSpace(cat)))
				}
			}
		}
	}

	isApproved := score <= 0.5
	result, err := entity.NewModerationResult(
		"",
		isApproved,
		score,
		categories,
		textModelName,
		0,
	)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func parseImageModerationResponse(resp *triton.ModelInferResponse) (*entity.ModerationResult, error) {
	return parseTextModerationResponse(resp)
}

var _ outbound.AIClient = (*TritonClient)(nil)
