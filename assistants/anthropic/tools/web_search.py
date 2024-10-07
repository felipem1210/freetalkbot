from googlesearch import search
import logger as log

logger = log.get_logger()


def web_search(topic):
    result = ""
    logger.info(f"Searching the web for {topic}")
    for j in search(topic, tld="co.in", num=2, stop=2, pause=2):
        result = j + ", " + result

    return f"You can find the results for {topic} in the following links: {result}"


def get_tools():
    return [
        {
            "name": "web_search",
            "description": "A tool to get information of a product by searching the web",
            "input_schema": {
                "type": "object",
                "properties": {
                    "topic": {
                        "type": "string",
                        "description": "The topic to search the web for",
                    },
                },
                "required": ["topic"],
            },
        }
    ]
