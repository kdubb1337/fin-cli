package config

import "fmt"

// ResolveItem returns the item-id to use based on:
//  1. --item flag (explicit)
//  2. --profile flag → profile's item-id
//  3. FIN_PROFILE env → profile's item-id
//  4. active_profile → profile's item-id
//  5. profile named "default" → profile's item-id
func (c *Config) ResolveItem(flagItem, flagProfile, envProfile string) (string, error) {
	if flagItem != "" {
		if _, ok := c.Items[flagItem]; !ok {
			return "", fmt.Errorf("item %q not found", flagItem)
		}
		return flagItem, nil
	}
	for _, name := range []string{flagProfile, envProfile, c.ActiveProfile, "default"} {
		if name == "" {
			continue
		}
		p, ok := c.Profiles[name]
		if !ok {
			continue
		}
		if _, ok := c.Items[p.ItemID]; !ok {
			return "", fmt.Errorf("profile %q references missing item %q", name, p.ItemID)
		}
		return p.ItemID, nil
	}
	return "", fmt.Errorf("no item or profile configured; run `fin auth add` to link an institution")
}
