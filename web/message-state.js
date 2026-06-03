export function upsertMessage(messages, message) {
  if (!message || message.messageId === undefined || message.messageId === null) {
    return messages;
  }
  return [
    message,
    ...messages.filter((item) => String(item.messageId) !== String(message.messageId)),
  ];
}

export function upsertMessageFromSendError(messages, error) {
  const message = error?.payload?.message;
  if (!message) {
    return messages;
  }
  return upsertMessage(messages, message);
}
