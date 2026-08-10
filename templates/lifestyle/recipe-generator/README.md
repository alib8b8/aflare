# Recipe Generator

> Generate personalized recipes from available ingredients

## Description

This workflow template provides a ready-to-use solution for generate personalized recipes from available ingredients.

## Usage

```bash
aflare install lifestyle/recipe-generator
aflare run recipe-generator/workflow.yaml
```

## Parameters

| Parameter | Description | Required |
|-----------|-------------|----------|
| ingredients | Available ingredients list | Yes |
| dietary_preferences | Dietary preferences (e.g., vegetarian, keto) | No |
| cuisine_type | Desired cuisine type | No |
| meal_type | Meal type (breakfast, lunch, dinner, snack) | Yes |
| cooking_time | Maximum cooking time | No |
| skill_level | Cooking skill level | No |
| servings | Number of servings | Yes |
| allergies | Food allergies or restrictions | No |

## Nodes Used

- agent - AI agent node for intelligent processing
- researcher - Research and information gathering
- template_render - Template rendering with Go templates
- file_write - Write output to files

## Category

lifestyle