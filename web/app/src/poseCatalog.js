export const poseDimensions = [
  ["gender", "适用"], ["shot", "景别"], ["action_tags", "动作"],
  ["style_tags", "风格"], ["clothing_tags", "服装"], ["scene", "场景"],
];
export function poseValues(pose, key) {
  const value = pose[key];
  return Array.isArray(value) ? value : value ? [value] : [];
}
export function filterPoses(poses, query, filters) {
  const terms = query.trim().toLocaleLowerCase().split(/\s+/).filter(Boolean);
  return poses.filter((pose) => {
    const text = [pose.id, pose.name, pose.instruction, pose.source_author, ...poseDimensions.flatMap(([key]) => poseValues(pose, key))].join(" ").toLocaleLowerCase();
    return terms.every((term) => text.includes(term)) && Object.entries(filters).every(([key, value]) => !value || poseValues(pose, key).includes(value));
  });
}
