export const NOT_IMPLEMENTED_MESSAGE = '该功能未实现';

export function notImplemented(_featureName, notify) {
  if (typeof notify === 'function') notify(NOT_IMPLEMENTED_MESSAGE);
  return NOT_IMPLEMENTED_MESSAGE;
}
