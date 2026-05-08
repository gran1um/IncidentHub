import type { WorkflowNodeTemplate } from "../types";

export const node: WorkflowNodeTemplate = {
  id: "telegram_send",
  title: "Telegram Send",
  description: "Send Telegram message via configured outbound connector",
  category: "Action",
  defaultConfig: {
    connectorId: "",
    recipient: "",
    message: "Incident update: {{input.case_id}}",
  },
  fields: [
    {
      id: "connectorId",
      label: "Telegram connector",
      type: "connector",
      description: "Choose outbound connector with channel=telegram",
    },
    {
      id: "recipient",
      label: "Recipient (@username or chat_id)",
      type: "text",
      placeholder: "@user or 123456789",
    },
    {
      id: "message",
      label: "Message template",
      type: "textarea",
      placeholder: "Case {{input.case_id}} updated",
    },
  ],
  summaryField: "recipient",
  summaryFallback: "recipient is not set",
};

