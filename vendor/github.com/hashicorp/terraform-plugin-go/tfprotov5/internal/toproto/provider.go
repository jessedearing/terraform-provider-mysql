package toproto

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5/internal/tfplugin5"
)

func GetMetadata_Request(in *tfprotov5.GetMetadataRequest) (*tfplugin5.GetMetadata_Request, error) {
	return &tfplugin5.GetMetadata_Request{}, nil
}

func GetMetadata_Response(in *tfprotov5.GetMetadataResponse) (*tfplugin5.GetMetadata_Response, error) {
	if in == nil {
		return nil, nil
	}

	resp := &tfplugin5.GetMetadata_Response{
		DataSources:        make([]*tfplugin5.GetMetadata_DataSourceMetadata, 0, len(in.DataSources)),
		Functions:          make([]*tfplugin5.GetMetadata_FunctionMetadata, 0, len(in.Functions)),
		Resources:          make([]*tfplugin5.GetMetadata_ResourceMetadata, 0, len(in.Resources)),
		ServerCapabilities: ServerCapabilities(in.ServerCapabilities),
	}

	for _, datasource := range in.DataSources {
		resp.DataSources = append(resp.DataSources, GetMetadata_DataSourceMetadata(&datasource))
	}

	for _, function := range in.Functions {
		resp.Functions = append(resp.Functions, GetMetadata_FunctionMetadata(&function))
	}

	for _, resource := range in.Resources {
		resp.Resources = append(resp.Resources, GetMetadata_ResourceMetadata(&resource))
	}

	diags, err := Diagnostics(in.Diagnostics)

	if err != nil {
		return resp, err
	}

	resp.Diagnostics = diags

	return resp, nil
}

func GetProviderSchema_Request(in *tfprotov5.GetProviderSchemaRequest) (*tfplugin5.GetProviderSchema_Request, error) {
	return &tfplugin5.GetProviderSchema_Request{}, nil
}

func GetProviderSchema_Response(in *tfprotov5.GetProviderSchemaResponse) (*tfplugin5.GetProviderSchema_Response, error) {
	if in == nil {
		return nil, nil
	}
	resp := tfplugin5.GetProviderSchema_Response{
		DataSourceSchemas:  make(map[string]*tfplugin5.Schema, len(in.DataSourceSchemas)),
		Functions:          make(map[string]*tfplugin5.Function, len(in.Functions)),
		ResourceSchemas:    make(map[string]*tfplugin5.Schema, len(in.ResourceSchemas)),
		ServerCapabilities: ServerCapabilities(in.ServerCapabilities),
	}
	if in.Provider != nil {
		schema, err := Schema(in.Provider)
		if err != nil {
			return &resp, fmt.Errorf("error marshaling provider schema: %w", err)
		}
		resp.Provider = schema
	}
	if in.ProviderMeta != nil {
		schema, err := Schema(in.ProviderMeta)
		if err != nil {
			return &resp, fmt.Errorf("error marshaling provider_meta schema: %w", err)
		}
		resp.ProviderMeta = schema
	}

	for k, v := range in.ResourceSchemas {
		if v == nil {
			resp.ResourceSchemas[k] = nil
			continue
		}
		schema, err := Schema(v)
		if err != nil {
			return &resp, fmt.Errorf("error marshaling resource schema for %q: %w", k, err)
		}
		resp.ResourceSchemas[k] = schema
	}

	for k, v := range in.DataSourceSchemas {
		if v == nil {
			resp.DataSourceSchemas[k] = nil
			continue
		}
		schema, err := Schema(v)
		if err != nil {
			return &resp, fmt.Errorf("error marshaling data source schema for %q: %w", k, err)
		}
		resp.DataSourceSchemas[k] = schema
	}

	for name, functionPtr := range in.Functions {
		if functionPtr == nil {
			resp.Functions[name] = nil
			continue
		}

		function, err := Function(functionPtr)

		if err != nil {
			return &resp, fmt.Errorf("error marshaling function definition for %q: %w", name, err)
		}

		resp.Functions[name] = function
	}

	diags, err := Diagnostics(in.Diagnostics)
	if err != nil {
		return &resp, err
	}
	resp.Diagnostics = diags
	return &resp, nil
}

func PrepareProviderConfig_Request(in *tfprotov5.PrepareProviderConfigRequest) (*tfplugin5.PrepareProviderConfig_Request, error) {
	resp := &tfplugin5.PrepareProviderConfig_Request{}
	if in.Config != nil {
		resp.Config = DynamicValue(in.Config)
	}
	return resp, nil
}

func PrepareProviderConfig_Response(in *tfprotov5.PrepareProviderConfigResponse) (*tfplugin5.PrepareProviderConfig_Response, error) {
	diags, err := Diagnostics(in.Diagnostics)
	if err != nil {
		return nil, err
	}
	resp := &tfplugin5.PrepareProviderConfig_Response{
		Diagnostics: diags,
	}
	if in.PreparedConfig != nil {
		resp.PreparedConfig = DynamicValue(in.PreparedConfig)
	}
	return resp, nil
}

func Configure_Request(in *tfprotov5.ConfigureProviderRequest) (*tfplugin5.Configure_Request, error) {
	resp := &tfplugin5.Configure_Request{
		TerraformVersion: in.TerraformVersion,
	}
	if in.Config != nil {
		resp.Config = DynamicValue(in.Config)
	}
	return resp, nil
}

func Configure_Response(in *tfprotov5.ConfigureProviderResponse) (*tfplugin5.Configure_Response, error) {
	diags, err := Diagnostics(in.Diagnostics)
	if err != nil {
		return nil, err
	}
	return &tfplugin5.Configure_Response{
		Diagnostics: diags,
	}, nil
}

func Stop_Request(in *tfprotov5.StopProviderRequest) (*tfplugin5.Stop_Request, error) {
	return &tfplugin5.Stop_Request{}, nil
}

func Stop_Response(in *tfprotov5.StopProviderResponse) (*tfplugin5.Stop_Response, error) {
	return &tfplugin5.Stop_Response{
		Error: in.Error,
	}, nil
}

// we have to say this next thing to get golint to stop yelling at us about the
// underscores in the function names. We want the function names to match
// actually-generated code, so it feels like fair play. It's just a shame we
// lose golint for the entire file.
//
// This file is not actually generated. You can edit it. Ignore this next line.
// Code generated by hand ignore this next bit DO NOT EDIT.
