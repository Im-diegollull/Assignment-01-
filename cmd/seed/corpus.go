package main

var firstNames = []string{
	"Adriana", "Alonso", "Amparo", "Aníbal", "Beatriz", "Benjamín", "Camila",
	"Casimiro", "Clara", "Damián", "Delfina", "Elías", "Emilia", "Ernesto",
	"Esperanza", "Fabián", "Florencia", "Gaspar", "Gertrudis", "Gonzalo",
	"Herminia", "Ignacio", "Inés", "Joaquín", "Julieta", "Leonor", "Lucía",
	"Mariana", "Manuel", "Marcela", "Mateo", "Micaela", "Nicanor", "Olegario",
	"Paloma", "Patricio", "Ramona", "Renato", "Rosario", "Salvador", "Serafina",
	"Silvestre", "Sofía", "Teodoro", "Tomás", "Valentina", "Vicente", "Ximena",
	"Yolanda", "Zacarías",
}

var lastNames = []string{
	"Alcaraz", "Arrieta", "Barros", "Bermúdez", "Cabrera", "Calderón", "Cifuentes",
	"Contreras", "Del Valle", "Echeverría", "Eguren", "Fuentealba", "Gallardo",
	"Herrera", "Ibarra", "Jaramillo", "Lagos", "Larraín", "Maldonado", "Mendoza",
	"Montalbán", "Narváez", "Olivares", "Ossandón", "Peralta", "Quintana",
	"Quiroga", "Rebolledo", "Riquelme", "Sandoval", "Sepúlveda", "Solís",
	"Tapia", "Ugarte", "Urrutia", "Valdivia", "Vergara", "Villalobos", "Yáñez",
	"Zamorano", "Zapata", "Zúñiga",
}

var countries = []string{
	"Chile", "Argentina", "México", "España", "Colombia", "Perú", "Uruguay",
	"Cuba", "Bolivia", "Ecuador", "Guatemala", "Venezuela", "Paraguay",
	"Costa Rica", "Portugal",
}

var authorRoles = []string{
	"novelista", "cuentista", "poeta y ensayista", "cronista",
	"periodista y novelista", "ensayista", "novelista y guionista",
	"cronista de viajes",
}

var authorThemes = []string{
	"la memoria familiar", "el exilio", "la vida en los puertos",
	"la burocracia y sus laberintos", "la infancia en el campo",
	"las ciudades que desaparecen", "el oficio de mirar", "las deudas heredadas",
	"los oficios olvidados", "la frontera y sus fantasmas",
	"el desierto y sus silencios", "la vida de provincia",
}

var authorTraits = []string{
	"Escribe frases cortas y finales abruptos.",
	"Publicó tarde y con una obra breve pero constante.",
	"Alterna la novela larga con el cuento mínimo.",
	"Trabajó veinte años en la docencia rural antes de publicar.",
	"Su obra fue traducida a nueve idiomas.",
	"Rechaza las entrevistas y firma con un solo apellido.",
	"Corrige cada libro durante años antes de entregarlo.",
	"Empezó escribiendo folletines para un diario de provincia.",
	"Su nombre se asocia con una generación que nunca reconoció como propia.",
	"Reúne su obra en volúmenes ilustrados por su propia mano.",
}

// Piezas para armar títulos.
var titleNouns = []string{
	"jardín", "naufragio", "invierno", "atlas", "cuaderno", "retrato", "espejo",
	"puente", "faro", "archivo", "desierto", "incendio", "eclipse", "silencio",
	"reloj", "mapa", "puerto", "umbral", "vértigo", "tejado", "andén", "sótano",
	"testamento", "inventario", "mordisco", "diluvio", "cortejo", "quiebre",
}

var titleModifiers = []string{
	"las cenizas", "los pájaros", "la memoria", "los relojes detenidos",
	"la sal", "las deudas", "los nombres perdidos", "la niebla", "los espejos",
	"las cosas mudas", "el olvido", "las manos ajenas", "los días iguales",
	"la casa vacía", "los trenes de carga", "la última hora", "las estaciones",
	"los que no volvieron", "la sombra larga", "el vidrio roto",
}

var titleAdjectives = []string{
	"Breve", "Última", "Pequeño", "Íntimo", "Áspero", "Lento", "Improbable",
	"Silencioso", "Provisorio", "Definitivo", "Oblicuo", "Sucesivo",
}

var titlePlaces = []string{
	"Valparaíso", "el sur", "la cordillera", "una isla sin nombre", "Bahía Negra",
	"el barrio Franklin", "la pampa", "Puerto Ceniza", "las afueras",
	"la costa rota", "San Cristóbal", "el kilómetro cero",
}

var summaryOpenings = []string{
	"Tras la muerte de su padre, %s regresa a %s para vender la casa familiar.",
	"Durante un verano interminable, %s descubre un archivo olvidado en %s.",
	"Una carta sin remitente obliga a %s a volver a %s después de treinta años.",
	"El derrumbe de un edificio deja a %s sin documentos ni dinero en %s.",
	"Cuando el río se lleva el puente, %s queda sin salida en %s junto a un desconocido.",
	"%s acepta catalogar la biblioteca de un hombre que acaba de morir en %s.",
	"Un naufragio frente a la costa devuelve a %s a la casa de %s que había jurado no pisar.",
	"%s trabaja de noche en una fábrica de %s y escribe en los márgenes de los formularios.",
}

var summaryMiddles = []string{
	"Lo que empieza como un trámite se convierte en una investigación sobre la memoria de un pueblo entero.",
	"Cada objeto que encuentra abre una versión distinta de la misma historia.",
	"Las conversaciones con los vecinos revelan un pacto de silencio de décadas.",
	"La rutina se quiebra cuando aparece un cuaderno con nombres tachados.",
	"Los días se ordenan alrededor de una espera que nadie sabe explicar.",
	"La novela avanza por acumulación de detalles mínimos y postergaciones.",
	"El relato alterna dos épocas separadas por cuarenta años y una traición.",
	"Entre inventarios y deudas impagas, la protagonista arma un mapa de su propia infancia.",
	"El narrador reconstruye los hechos a partir de recibos, fotografías y rumores.",
	"Un incendio en el archivo municipal borra la única prueba que quedaba.",
}

var summaryClosings = []string{
	"Una novela sobre lo que se hereda sin querer.",
	"Un libro breve sobre la distancia entre lo que ocurrió y lo que se cuenta.",
	"El final no ofrece consuelo, pero sí una forma de orden.",
	"Una historia sobre el olvido como forma de supervivencia.",
	"Un retrato del desarraigo escrito sin nostalgia.",
	"Una meditación sobre el trabajo, la deuda y el paso del tiempo.",
	"La obra más ambiciosa de su autor hasta la fecha.",
	"Un relato sobre la culpa que se transmite de una generación a otra.",
	"Una novela sobre el regreso y su imposibilidad.",
	"Un ejercicio de memoria contra el ruido de los archivos.",
}

var summaryCharacters = []string{
	"una topógrafa", "un tipógrafo jubilado", "una archivista", "un ferroviario",
	"una médica rural", "un tasador de seguros", "una fotógrafa", "un relojero",
	"una traductora", "un cartero", "una bibliotecaria", "un afinador de pianos",
	"una geóloga", "un guardafaro", "una notaria", "un afilador ambulante",
}

// Reseñas por tramo de puntaje, para que el texto sea coherente con la nota.
var reviewsByScore = map[int][]string{
	1: {
		"No pude terminarlo. Trescientas páginas para llegar a ninguna parte.",
		"Prosa pretenciosa y personajes de cartón. Una decepción.",
		"El planteo prometía, la ejecución es un desastre.",
		"Se nota el apuro de la editorial. Necesitaba tres correcciones más.",
	},
	2: {
		"Tiene momentos, pero se pierde en digresiones interminables.",
		"La primera mitad funciona; la segunda se desarma sin remedio.",
		"Demasiados personajes para tan poca historia.",
		"Le sobran cien páginas y le falta un final.",
	},
	3: {
		"Correcto, sin más. Se lee rápido y se olvida igual de rápido.",
		"Buenas ideas mal distribuidas. Rescato el capítulo del archivo.",
		"Cumple lo que promete, aunque no sorprenda en ningún momento.",
		"Un libro digno, ni memorable ni malo.",
	},
	4: {
		"Muy buena novela. El manejo del tiempo narrativo es notable.",
		"Me atrapó desde el primer capítulo. Le falta poco para ser redonda.",
		"Los secundarios están mejor construidos que el protagonista, y eso es un elogio.",
		"Una lectura sólida, con un final que sostiene todo lo anterior.",
	},
	5: {
		"Extraordinario. Lo terminé de madrugada y volví a empezarlo.",
		"De lo mejor que leí en años. Cada frase está donde tiene que estar.",
		"Una obra maestra silenciosa. La recomiendo sin reservas.",
		"No conocía a este autor y ya encargué todo lo demás que escribió.",
		"Un libro que reordena la forma en que uno mira su propia familia.",
	},
}
