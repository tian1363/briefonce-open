// Package imagepolicy shares visual requirements across image workflows.
package imagepolicy

// NaturalSkin changes rendering quality, never the subject's identity or skin tone.
const NaturalSkin = "画面中有人物时，脸部呈现自然清爽的真实肤质，保留细微毛孔、皮肤纹理及原有肤色、年龄和五官；使用柔和漫射光与克制的局部高光。减少额头、鼻梁、鼻翼和面颊的大面积油亮反光，避免油腻面部、镜面反光、湿亮妆感、蜡像感、塑料感和过度磨皮；不美白、不改脸，不为改善肤质改变人物身份。"

const UnnaturalSkin = "油腻面部、大面积油光、镜面反光、湿亮皮肤、蜡像脸、塑料皮肤、过度磨皮、肤色漂白"
