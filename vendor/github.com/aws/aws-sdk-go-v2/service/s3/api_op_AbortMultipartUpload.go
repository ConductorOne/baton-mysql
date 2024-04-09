// Code generated by smithy-go-codegen DO NOT EDIT.

package s3

import (
	"context"
	"fmt"
	awsmiddleware "github.com/aws/aws-sdk-go-v2/aws/middleware"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	s3cust "github.com/aws/aws-sdk-go-v2/service/s3/internal/customizations"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// This action aborts a multipart upload. After a multipart upload is aborted, no
// additional parts can be uploaded using that upload ID. The storage consumed by
// any previously uploaded parts will be freed. However, if any part uploads are
// currently in progress, those part uploads might or might not succeed. As a
// result, it might be necessary to abort a given multipart upload multiple times
// in order to completely free all storage consumed by all parts. To verify that
// all parts have been removed, so you don't get charged for the part storage, you
// should call the ListParts (https://docs.aws.amazon.com/AmazonS3/latest/API/API_ListParts.html)
// action and ensure that the parts list is empty. For information about
// permissions required to use the multipart upload, see Multipart Upload and
// Permissions (https://docs.aws.amazon.com/AmazonS3/latest/dev/mpuAndPermissions.html)
// . The following operations are related to AbortMultipartUpload :
//   - CreateMultipartUpload (https://docs.aws.amazon.com/AmazonS3/latest/API/API_CreateMultipartUpload.html)
//   - UploadPart (https://docs.aws.amazon.com/AmazonS3/latest/API/API_UploadPart.html)
//   - CompleteMultipartUpload (https://docs.aws.amazon.com/AmazonS3/latest/API/API_CompleteMultipartUpload.html)
//   - ListParts (https://docs.aws.amazon.com/AmazonS3/latest/API/API_ListParts.html)
//   - ListMultipartUploads (https://docs.aws.amazon.com/AmazonS3/latest/API/API_ListMultipartUploads.html)
func (c *Client) AbortMultipartUpload(ctx context.Context, params *AbortMultipartUploadInput, optFns ...func(*Options)) (*AbortMultipartUploadOutput, error) {
	if params == nil {
		params = &AbortMultipartUploadInput{}
	}

	result, metadata, err := c.invokeOperation(ctx, "AbortMultipartUpload", params, optFns, c.addOperationAbortMultipartUploadMiddlewares)
	if err != nil {
		return nil, err
	}

	out := result.(*AbortMultipartUploadOutput)
	out.ResultMetadata = metadata
	return out, nil
}

type AbortMultipartUploadInput struct {

	// The bucket name to which the upload was taking place. When using this action
	// with an access point, you must direct requests to the access point hostname. The
	// access point hostname takes the form
	// AccessPointName-AccountId.s3-accesspoint.Region.amazonaws.com. When using this
	// action with an access point through the Amazon Web Services SDKs, you provide
	// the access point ARN in place of the bucket name. For more information about
	// access point ARNs, see Using access points (https://docs.aws.amazon.com/AmazonS3/latest/userguide/using-access-points.html)
	// in the Amazon S3 User Guide. When you use this action with Amazon S3 on
	// Outposts, you must direct requests to the S3 on Outposts hostname. The S3 on
	// Outposts hostname takes the form
	// AccessPointName-AccountId.outpostID.s3-outposts.Region.amazonaws.com . When you
	// use this action with S3 on Outposts through the Amazon Web Services SDKs, you
	// provide the Outposts access point ARN in place of the bucket name. For more
	// information about S3 on Outposts ARNs, see What is S3 on Outposts? (https://docs.aws.amazon.com/AmazonS3/latest/userguide/S3onOutposts.html)
	// in the Amazon S3 User Guide.
	//
	// This member is required.
	Bucket *string

	// Key of the object for which the multipart upload was initiated.
	//
	// This member is required.
	Key *string

	// Upload ID that identifies the multipart upload.
	//
	// This member is required.
	UploadId *string

	// The account ID of the expected bucket owner. If the bucket is owned by a
	// different account, the request fails with the HTTP status code 403 Forbidden
	// (access denied).
	ExpectedBucketOwner *string

	// Confirms that the requester knows that they will be charged for the request.
	// Bucket owners need not specify this parameter in their requests. If either the
	// source or destination Amazon S3 bucket has Requester Pays enabled, the requester
	// will pay for corresponding charges to copy the object. For information about
	// downloading objects from Requester Pays buckets, see Downloading Objects in
	// Requester Pays Buckets (https://docs.aws.amazon.com/AmazonS3/latest/dev/ObjectsinRequesterPaysBuckets.html)
	// in the Amazon S3 User Guide.
	RequestPayer types.RequestPayer

	noSmithyDocumentSerde
}

func (in *AbortMultipartUploadInput) bindEndpointParams(p *EndpointParameters) {
	p.Bucket = in.Bucket

}

type AbortMultipartUploadOutput struct {

	// If present, indicates that the requester was successfully charged for the
	// request.
	RequestCharged types.RequestCharged

	// Metadata pertaining to the operation's result.
	ResultMetadata middleware.Metadata

	noSmithyDocumentSerde
}

func (c *Client) addOperationAbortMultipartUploadMiddlewares(stack *middleware.Stack, options Options) (err error) {
	if err := stack.Serialize.Add(&setOperationInputMiddleware{}, middleware.After); err != nil {
		return err
	}
	err = stack.Serialize.Add(&awsRestxml_serializeOpAbortMultipartUpload{}, middleware.After)
	if err != nil {
		return err
	}
	err = stack.Deserialize.Add(&awsRestxml_deserializeOpAbortMultipartUpload{}, middleware.After)
	if err != nil {
		return err
	}
	if err := addProtocolFinalizerMiddlewares(stack, options, "AbortMultipartUpload"); err != nil {
		return fmt.Errorf("add protocol finalizers: %v", err)
	}

	if err = addlegacyEndpointContextSetter(stack, options); err != nil {
		return err
	}
	if err = addSetLoggerMiddleware(stack, options); err != nil {
		return err
	}
	if err = awsmiddleware.AddClientRequestIDMiddleware(stack); err != nil {
		return err
	}
	if err = smithyhttp.AddComputeContentLengthMiddleware(stack); err != nil {
		return err
	}
	if err = addResolveEndpointMiddleware(stack, options); err != nil {
		return err
	}
	if err = v4.AddComputePayloadSHA256Middleware(stack); err != nil {
		return err
	}
	if err = addRetryMiddlewares(stack, options); err != nil {
		return err
	}
	if err = awsmiddleware.AddRawResponseToMetadata(stack); err != nil {
		return err
	}
	if err = awsmiddleware.AddRecordResponseTiming(stack); err != nil {
		return err
	}
	if err = addClientUserAgent(stack, options); err != nil {
		return err
	}
	if err = smithyhttp.AddErrorCloseResponseBodyMiddleware(stack); err != nil {
		return err
	}
	if err = smithyhttp.AddCloseResponseBodyMiddleware(stack); err != nil {
		return err
	}
	if err = addSetLegacyContextSigningOptionsMiddleware(stack); err != nil {
		return err
	}
	if err = addOpAbortMultipartUploadValidationMiddleware(stack); err != nil {
		return err
	}
	if err = stack.Initialize.Add(newServiceMetadataMiddleware_opAbortMultipartUpload(options.Region), middleware.Before); err != nil {
		return err
	}
	if err = addMetadataRetrieverMiddleware(stack); err != nil {
		return err
	}
	if err = awsmiddleware.AddRecursionDetection(stack); err != nil {
		return err
	}
	if err = addAbortMultipartUploadUpdateEndpoint(stack, options); err != nil {
		return err
	}
	if err = addResponseErrorMiddleware(stack); err != nil {
		return err
	}
	if err = v4.AddContentSHA256HeaderMiddleware(stack); err != nil {
		return err
	}
	if err = disableAcceptEncodingGzip(stack); err != nil {
		return err
	}
	if err = addRequestResponseLogging(stack, options); err != nil {
		return err
	}
	if err = addDisableHTTPSMiddleware(stack, options); err != nil {
		return err
	}
	if err = addSerializeImmutableHostnameBucketMiddleware(stack, options); err != nil {
		return err
	}
	return nil
}

func (v *AbortMultipartUploadInput) bucket() (string, bool) {
	if v.Bucket == nil {
		return "", false
	}
	return *v.Bucket, true
}

func newServiceMetadataMiddleware_opAbortMultipartUpload(region string) *awsmiddleware.RegisterServiceMetadata {
	return &awsmiddleware.RegisterServiceMetadata{
		Region:        region,
		ServiceID:     ServiceID,
		OperationName: "AbortMultipartUpload",
	}
}

// getAbortMultipartUploadBucketMember returns a pointer to string denoting a
// provided bucket member valueand a boolean indicating if the input has a modeled
// bucket name,
func getAbortMultipartUploadBucketMember(input interface{}) (*string, bool) {
	in := input.(*AbortMultipartUploadInput)
	if in.Bucket == nil {
		return nil, false
	}
	return in.Bucket, true
}
func addAbortMultipartUploadUpdateEndpoint(stack *middleware.Stack, options Options) error {
	return s3cust.UpdateEndpoint(stack, s3cust.UpdateEndpointOptions{
		Accessor: s3cust.UpdateEndpointParameterAccessor{
			GetBucketFromInput: getAbortMultipartUploadBucketMember,
		},
		UsePathStyle:                   options.UsePathStyle,
		UseAccelerate:                  options.UseAccelerate,
		SupportsAccelerate:             true,
		TargetS3ObjectLambda:           false,
		EndpointResolver:               options.EndpointResolver,
		EndpointResolverOptions:        options.EndpointOptions,
		UseARNRegion:                   options.UseARNRegion,
		DisableMultiRegionAccessPoints: options.DisableMultiRegionAccessPoints,
	})
}
