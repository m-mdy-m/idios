package ai

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// ProviderAdapterDescriptor is the safe, public metadata IDIOS exposes for an
// adapter implementation. It deliberately does not expose adapter configuration
// or secrets.
type ProviderAdapterDescriptor struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Version     string `json:"version,omitempty"`
	Description string `json:"description,omitempty"`
	Protocol    string `json:"protocol,omitempty"`
	BuiltIn     bool   `json:"built_in"`
}

// ProviderAdapterFactory constructs a provider implementation for a saved
// ProviderConfig. The adapter is responsible for translating the shared AI
// contract into its native endpoint/protocol.
type ProviderAdapterFactory func(ProviderConfig) (Provider, error)

type registeredProviderAdapter struct {
	descriptor ProviderAdapterDescriptor
	factory    ProviderAdapterFactory
}

var adapterRegistry = struct {
	sync.RWMutex
	items map[string]registeredProviderAdapter
}{items: make(map[string]registeredProviderAdapter)}

var builtinAdapterDescriptors = []ProviderAdapterDescriptor{
	{ID: "openrouter", Name: "OpenRouter", Version: "1", Description: "OpenRouter API", Protocol: "openai-compatible", BuiltIn: true},
	{ID: "openai-compatible", Name: "OpenAI-compatible", Version: "1", Description: "OpenAI-compatible chat completions and model catalogue", Protocol: "openai-compatible", BuiltIn: true},
	{ID: "ollama-native", Name: "Ollama native", Version: "1", Description: "Ollama /api/tags and /api/chat", Protocol: "ollama-native", BuiltIn: true},
}

// RegisterProviderAdapter registers a custom adapter implemented in Go. Adapters
// are linked into the IDIOS executable at build time; this is portable across
// Windows, macOS and Linux unlike Go's platform-sensitive plugin mechanism.
// Registration is usually performed from the extension package's init function.
func RegisterProviderAdapter(descriptor ProviderAdapterDescriptor, factory ProviderAdapterFactory) error {
	descriptor.ID = strings.TrimSpace(strings.ToLower(descriptor.ID))
	descriptor.Name = strings.TrimSpace(descriptor.Name)
	if len(descriptor.ID) < 2 || len(descriptor.ID) > 64 || descriptor.Name == "" || factory == nil {
		return fmt.Errorf("provider adapter requires an ID, a name, and a factory")
	}
	for _, r := range descriptor.ID {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.') {
			return fmt.Errorf("provider adapter ID must contain only lowercase letters, digits, '.', '_' or '-'")
		}
	}
	for _, builtin := range builtinAdapterDescriptors {
		if builtin.ID == descriptor.ID {
			return fmt.Errorf("provider adapter ID %q is reserved by IDIOS", descriptor.ID)
		}
	}
	adapterRegistry.Lock()
	defer adapterRegistry.Unlock()
	if _, exists := adapterRegistry.items[descriptor.ID]; exists {
		return fmt.Errorf("provider adapter %q is already registered", descriptor.ID)
	}
	descriptor.BuiltIn = false
	adapterRegistry.items[descriptor.ID] = registeredProviderAdapter{descriptor: descriptor, factory: factory}
	return nil
}

// AvailableProviderAdapters returns public descriptors for built-in and
// extension adapters, sorted by display name.
func AvailableProviderAdapters() []ProviderAdapterDescriptor {
	items := append([]ProviderAdapterDescriptor(nil), builtinAdapterDescriptors...)
	adapterRegistry.RLock()
	for _, item := range adapterRegistry.items {
		items = append(items, item.descriptor)
	}
	adapterRegistry.RUnlock()
	sort.Slice(items, func(i, j int) bool {
		if items[i].Name == items[j].Name {
			return items[i].ID < items[j].ID
		}
		return items[i].Name < items[j].Name
	})
	return items
}

func registeredProviderFactory(id string) (ProviderAdapterFactory, bool) {
	adapterRegistry.RLock()
	item, ok := adapterRegistry.items[strings.TrimSpace(strings.ToLower(id))]
	adapterRegistry.RUnlock()
	return item.factory, ok
}

func providerAdapterRegistered(id string) bool {
	_, ok := registeredProviderFactory(id)
	return ok
}
