package tco_vo_agent

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var nowFn = time.Now

const moreInfoRequiredMessage = `Subject: TCO removal order – clarification required (Ref: %s)

Hello %s,

We received your removal order under Regulation (EU) 2021/784 dated %s. To comply with Article 3, we need more detail before the one-hour deadline can run. Missing information: %s.

Please provide:
- the exact URL(s) / message ID(s) or copies of the content;
- the relevant account identifier(s) (username, email, user ID) or profile link;
- the signed removal order (Annex I) including statement of reasons and legal basis;
- the order's reference number and contact for follow-up;
- whether confidentiality under Article 11(3) applies.

Under Article 3(8), the one-hour deadline resumes once we receive the clarification. We will process the order immediately and confirm via Annex II if requested.
`

const userNotFoundMessage = `Subject: TCO removal order – content not located (Ref: %s)

Hello %s,

We tried to act on your removal order under Article 3 but could not locate the account/content using the provided identifiers (%s). To resume the one-hour deadline (Article 3(8)), please send:

- exact URL(s) or message ID(s);
- current profile link or user ID and any recent username/email changes;
- screenshot or copy of the material with time/timezone captured;
- whether confidentiality under Article 11(3) applies.

No further action has been taken until we receive the above.
`

const userBannedMessage = `Subject: TCO removal order – action completed (Ref: %s)

Hello %s,

We executed the removal order under Article 3 of Regulation (EU) 2021/784. Access to the reported account/content (%s) has been disabled across our service as of %s UTC.

We have preserved the removed content and related data for six months in line with Article 6 and can extend retention on request for ongoing proceedings. If you need confirmation in the Annex II format, please let us know.

Thank you.
`

func buildMessage(template string, data agentData) (string, error) {
	agency := fallbackValue(data.Data.AgencyName, "competent authority")
	reference := fallbackValue(data.Data.ReferenceNumber, "N/A")

	switch template {
	case "more_info_required":
		orderDate := fallbackValue(data.Data.Date, "not provided")
		missing := fallbackValue(strings.TrimSpace(data.Reason), "Additional identifiers required under Article 3(4) to locate the content.")
		return fmt.Sprintf(moreInfoRequiredMessage, reference, agency, orderDate, missing), nil
	case "user_not_found":
		identifiers := formatIdentifiers(data.Data)
		return fmt.Sprintf(userNotFoundMessage, reference, agency, identifiers), nil
	case "user_banned":
		identifiers := formatIdentifiers(data.Data)
		actionTime := nowFn().UTC().Format(time.RFC3339)
		return fmt.Sprintf(userBannedMessage, reference, agency, identifiers, actionTime), nil
	default:
		return "", errors.New("invalid message template")
	}
}

func fallbackValue(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func formatIdentifiers(decision FraudDecision) string {
	var parts []string
	if decision.Username != "" {
		parts = append(parts, fmt.Sprintf("username: %s", decision.Username))
	}
	if decision.Email != "" {
		parts = append(parts, fmt.Sprintf("email: %s", decision.Email))
	}
	if len(parts) == 0 {
		return "no user identifier provided"
	}
	return strings.Join(parts, " / ")
}
