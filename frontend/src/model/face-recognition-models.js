import $api from "common/api";

const resource = "faces/models";

// list returns face recognition model availability for this instance.
export function list() {
  return $api.get(resource).then((response) => response.data);
}

// get returns availability for one face recognition model.
export function get(modelKey) {
  return $api.get(`${resource}/${modelKey}`).then((response) => response.data);
}

// recognize prepares candidate embeddings and clusters for a model.
export function recognize(modelKey, force = false) {
  return $api.post(`${resource}/${modelKey}/recognize`, null, { params: { force } }).then((response) => response.data);
}

// compare returns candidate-vs-active model metrics.
export function compare(modelKey) {
  return $api.get(`${resource}/${modelKey}/compare`).then((response) => response.data);
}

// promote makes a prepared candidate the active recognition model.
export function promote(modelKey) {
  return $api.post(`${resource}/${modelKey}/promote`).then((response) => response.data);
}

export default {
  list,
  get,
  recognize,
  compare,
  promote,
};
