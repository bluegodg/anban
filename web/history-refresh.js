export async function loadConversationHistory({
  fetchHistory,
  currentMessages = [],
  preserveOnFailure = true,
}) {
  try {
    const payload = await fetchHistory();
    return {
      loaded: true,
      messages: Array.isArray(payload?.messages) ? payload.messages : [],
    };
  } catch {
    return {
      loaded: false,
      messages: preserveOnFailure && Array.isArray(currentMessages) ? currentMessages : [],
    };
  }
}
