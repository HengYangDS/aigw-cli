package domain

import "slices"

// RemoveRecommendedModel drops the model that ref names from every client's
// recommendation, making the first remaining alternative primary when the
// primary goes, and dropping a recommendation that has no choice left. It
// returns the clients whose recommendation changed, in sorted order.
func (c *Config) RemoveRecommendedModel(ref ModelRef) []string {
	changed := []string{}
	for client, recommendation := range c.Recommendations {
		before := len(recommendation.Alternatives)
		recommendation.Alternatives = slices.DeleteFunc(recommendation.Alternatives, func(selection ClientSelection) bool {
			return selection.Model == ref
		})
		switch {
		case recommendation.Primary.Model != ref:
			if len(recommendation.Alternatives) == before {
				continue
			}
			c.Recommendations[client] = recommendation
		case len(recommendation.Alternatives) == 0:
			delete(c.Recommendations, client)
		default:
			recommendation.Primary = recommendation.Alternatives[0]
			recommendation.Alternatives = recommendation.Alternatives[1:]
			c.Recommendations[client] = recommendation
		}
		changed = append(changed, client)
	}
	slices.Sort(changed)
	return changed
}
