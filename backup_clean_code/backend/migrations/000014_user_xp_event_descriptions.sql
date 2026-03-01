ALTER TABLE user_xp_events
    ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT '';

UPDATE user_xp_events
SET description = CASE
    WHEN event_type = 'case_closed' THEN
        'Closed case (' || COALESCE(NULLIF(details->>'severity', ''), 'medium') || ' severity)'
    WHEN event_type = 'achievement_granted' THEN
        'Achievement granted'
    WHEN event_type = 'connector_invoked' THEN
        'Connector invoked'
    WHEN event_type = 'analyzer_invoked' THEN
        'Analyzer invoked'
    WHEN event_type = 'responder_invoked' THEN
        'Responder invoked'
    WHEN event_type = 'incident_first_message' THEN
        'First incident message sent'
    WHEN event_type = 'manual_grant' THEN
        'Manual experience grant'
    ELSE
        'Experience rewarded'
END
WHERE COALESCE(description, '') = '';
