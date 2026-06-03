const PROFILE_FORM_FIELDS = [
  ['elderName', 'name'],
  ['nickname', 'nickname'],
  ['children', 'children'],
  ['grandchildren', 'grandchildren'],
  ['hobby', 'hobbies'],
  ['schedule', 'schedule'],
  ['health', 'health'],
  ['taboos', 'taboos'],
];

export function writeProfileFormFields(form, fields = {}) {
  for (const [inputName, fieldName] of PROFILE_FORM_FIELDS) {
    writeFormValue(form, inputName, fields[fieldName]);
  }
}

function writeFormValue(form, name, value) {
  const input = form?.elements?.namedItem(name);
  if (!input) return;
  input.value = Array.isArray(value) ? value.join(', ') : (value || '');
}
