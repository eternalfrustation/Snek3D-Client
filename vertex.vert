#version 410 core 
in vec3 aPos;
in vec4 aCol;
in vec3 aNor;
in vec2 aTex;
in float thresholdIn;
uniform mat4 projection;
uniform mat4 view;
uniform mat4 model;
out vec4 Col;
out vec3 Nor;
out vec2 TexCoords;
flat out float Threshold;
void main() {
	gl_Position = projection * view * model * vec4(aPos, 1.0);
	Col = aCol;
	Nor = aNor;
	TexCoords = aTex;
	Threshold = thresholdIn;
}
