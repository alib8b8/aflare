# Personalized Nutrition & Diet Plan

> Generate personalized nutrition and diet plans with macro calculations

## Description

This workflow template creates comprehensive nutrition plans based on patient metrics, medical conditions, dietary preferences, and health goals. It calculates BMI, BMR, TDEE, and macronutrient targets using evidence-based formulas, queries USDA food databases, and generates a detailed 7-day meal plan with grocery lists and meal prep tips.

## Usage

```bash
aflare install healthcare/nutrition-plan
aflare run nutrition-plan/workflow.yaml
```

## Parameters

| Parameter | Description | Required |
|-----------|-------------|----------|
| age | Patient age | Yes |
| gender | Patient gender | Yes |
| height_cm | Height in centimeters | Yes |
| weight_kg | Weight in kilograms | Yes |
| activity_level | sedentary, light, moderate, active, very_active | Yes |
| dietary_preferences | vegan, vegetarian, keto, mediterranean, etc. | No |
| food_allergies | Known food allergies | No |
| medical_conditions | Relevant conditions (diabetes, HTN, etc.) | No |
| health_goals | weight loss, muscle gain, maintenance, etc. | Yes |
| current_diet | Description of current eating habits | No |
| usda_api_key | USDA FoodData Central API key | No |
| output_path | Output file path | No |

## Nodes Used

- agent - AI agent for nutritional assessment and meal planning
- code_interpreter - BMI, BMR, TDEE, and macronutrient calculation
- http_request - USDA food database lookup
- file_write - Save nutrition plan

## Category

healthcare