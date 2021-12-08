#version 410 core
#define PI 3.1415
in vec4 Col;
in vec3 Nor;
in vec2 TexCoords;
flat in float Threshold;
void main() {
	vec4 color;
	color = Col;
	float dotprod = length(TexCoords);
	color = color; /* * (-atan(16*(dotprod - Threshold)/(1-Threshold))/PI + 0.5); */
	gl_FragColor = color;
}
