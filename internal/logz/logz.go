package logz

import "go.uber.org/zap"

func Error(err error) zap.Field {
	return zap.Error(err)
}

func SSHHostKey(key string) zap.Field {
	return zap.String("ssh_host_key", key)
}

func WorkspaceName(name string) zap.Field {
	return zap.String("workspace_name", name)
}

func WorkspaceURL(url string) zap.Field {
	return zap.String("workspace_url", url)
}

func WorkspaceHostTemplate(template string) zap.Field {
	return zap.String("workspace_host_template", template)
}

func WorkspaceHostTemplateData(data map[string]string) zap.Field {
	return zap.Any("workspace_host_template_data", data)
}

func ServiceName(name string) zap.Field {
	return zap.String("service_name", name)
}

func ServiceNamespace(namespace string) zap.Field {
	return zap.String("service_namespace", namespace)
}

func HostMappingHostname(hostname string) zap.Field {
	return zap.String("host_mapping_hostname", hostname)
}

func HostMappingBackend(backend string) zap.Field {
	return zap.String("host_mapping_backend", backend)
}

func HostMappingBackendPort(port int32) zap.Field {
	return zap.Int32("host_mapping_backend_port", port)
}

func HostMappingBackendProtocol(protocol string) zap.Field {
	return zap.String("host_mapping_protocol", protocol)
}

func HTTPPath(path string) zap.Field {
	return zap.String("http_path", path)
}

func HTTPIp(ip string) zap.Field {
	return zap.String("http_ip", ip)
}

func HTTPStatus(status int) zap.Field {
	return zap.Int("http_status", status)
}

func HTTPHost(host string) zap.Field {
	return zap.String("http_host", host)
}

func HTTPMethod(method string) zap.Field {
	return zap.String("http_method", method)
}

func HTTPScheme(scheme string) zap.Field {
	return zap.String("http_scheme", scheme)
}

func Port(port int) zap.Field {
	return zap.Int("port", port)
}
