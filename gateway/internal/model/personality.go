package model

type PersonalityType string

const (
	PersonalityExplorer     PersonalityType = "EXPLORER"
	PersonalityVisionary    PersonalityType = "VISIONARY"
	PersonalityStrategist   PersonalityType = "STRATEGIST"
	PersonalityInventor     PersonalityType = "INVENTOR"
	PersonalityDreamer      PersonalityType = "DREAMER"
	PersonalityCurator      PersonalityType = "CURATOR"
	PersonalityInspirer     PersonalityType = "INSPIRER"
	PersonalityDebater      PersonalityType = "DEBATER"
	PersonalityOrganizer    PersonalityType = "ORGANIZER"
	PersonalityConnector    PersonalityType = "CONNECTOR"
	PersonalityCompanion    PersonalityType = "COMPANION"
	PersonalityDriver       PersonalityType = "DRIVER"
	PersonalityAnchor       PersonalityType = "ANCHOR"
	PersonalityCraftsperson PersonalityType = "CRAFTSPERSON"
	PersonalityObserver     PersonalityType = "OBSERVER"
	PersonalityKeeper       PersonalityType = "KEEPER"
)

func (p PersonalityType) Description() string {
	switch p {
	case PersonalityExplorer:
		return "Исследователь: вам близки новые впечатления, активное общение и свобода действий; перемены обычно не выбивают вас из равновесия."
	case PersonalityVisionary:
		return "Визионер: вас привлекают новые возможности и обмен идеями; происходящее может вызывать у вас сильный эмоциональный отклик."
	case PersonalityStrategist:
		return "Стратег: вам ближе новые идеи, продуманный подход и спокойный формат общения."
	case PersonalityInventor:
		return "Изобретатель: вам нравится самостоятельно исследовать новые идеи, оставляя пространство для импровизации."
	case PersonalityDreamer:
		return "Мечтатель: вам близки воображение, гибкость и бережное общение; переживания занимают заметное место в вашей жизни."
	case PersonalityCurator:
		return "Куратор идей: вы сочетаете интерес к новому с организованностью и вниманием к людям, предпочитая спокойное общение."
	case PersonalityInspirer:
		return "Вдохновитель: вам близки новые идеи, активное общение и сотрудничество; в сложных ситуациях вы склонны сохранять спокойствие."
	case PersonalityDebater:
		return "Дискуссионер: вам нравится обсуждать новые идеи, отстаивать свою позицию и последовательно воплощать планы."
	case PersonalityOrganizer:
		return "Организатор: вам близки понятные задачи, порядок и активное взаимодействие с людьми."
	case PersonalityConnector:
		return "Объединитель: общение и сотрудничество важны для вас; события и отношения могут вызывать сильные переживания."
	case PersonalityCompanion:
		return "Компаньон: вам близки дружелюбное общение, знакомые занятия и свободный ритм без жёстких планов."
	case PersonalityDriver:
		return "Инициатор: вы склонны активно выражать свою позицию и действовать спонтанно, эмоционально откликаясь на происходящее."
	case PersonalityAnchor:
		return "Опора: вам близки устойчивый распорядок, внимательное отношение к людям и спокойное общение."
	case PersonalityCraftsperson:
		return "Мастер: вам ближе самостоятельная работа, проверенные подходы и последовательность в делах."
	case PersonalityObserver:
		return "Наблюдатель: вы предпочитаете сдержанное общение, сохраняете независимость суждений и чутко реагируете на происходящее."
	case PersonalityKeeper:
		return "Хранитель: вам важны привычный уклад, порядок и забота о близких; неопределённость может вызывать переживания."
	default:
		return ""
	}
}
