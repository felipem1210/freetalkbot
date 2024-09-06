import os
from anthropic import Anthropic
from anthropic.types import MessageParam
from datetime import date
from tools import web_search, customer_info
import logger as log

if os.getenv("ANTHROPIC_TOKEN") is None:
    raise ValueError("Please set the ANTHROPIC_TOKEN environment variable")

if os.getenv("ANTHROPIC_MODEL") is None:
    raise ValueError("Please set the ANTHROPIC_MODEL environment variable")

client = Anthropic(
    api_key=os.getenv("ANTHROPIC_TOKEN"),
)
MODEL_NAME = os.getenv("ANTHROPIC_MODEL")
MAX_TOKENS = 1000


def anthropic_chat(system_prompt, messages, enable_prompt_caching):
    logger = log.get_logger()
    logger.info("Claude is chatting with the user")

    if enable_prompt_caching:
        response = client.beta.prompt_caching.messages.create(
            system=system_prompt if system_prompt != "" else None,
            model=MODEL_NAME,
            messages=messages,
            max_tokens=MAX_TOKENS,
            tool_choice={"type": "any"},
            tools=customer_info.get_tools() + web_search.get_tools(),
        )
    else:
        response = client.messages.create(
            system=system_prompt if system_prompt != "" else None,
            model=MODEL_NAME,
            messages=messages,
            max_tokens=MAX_TOKENS,
            tool_choice={"type": "any"},
            tools=customer_info.get_tools() + web_search.get_tools(),
        )

    if response.stop_reason == "tool_use":
        last_content_block = response.content[-1]
        if last_content_block.type == "tool_use":
            tool_name = last_content_block.name
            tool_inputs = last_content_block.input
            logger.info(f"=======Claude Wants To Call The {tool_name} Tool=======")
            if tool_name == "send_text_to_user":
                message = customer_info.send_text_to_user(tool_inputs["text"])
            elif tool_name == "get_customer_info":
                message = customer_info.get_customer_info(tool_inputs["email"])
            elif tool_name == "web_search":
                message = web_search.web_search(tool_inputs["topic"])
            else:
                logger.info("Oh dear, that tool doesn't exist!")

    else:
        logger.info("No tool was called. This shouldn't happen!")
    return message


def chat_with_user(user_query, enable_prompt_caching):
    print(f"User asked: {user_query}")
    messages: MessageParam = {
        "role": "user",
        "content": user_query,
    }

    system_prompt = f"""
        You are a customer service assistant of a tech products seller company.
        All your communication with a user is done via text message.
        The user should ask only about order information or consult about tech products, if you detect that the user wants to talk about something not related to this,
        try to turn the conversation around. This is important.
        Use the same language that the user uses. This is important.

        Use web_search tool if user consults prices of tech products. This is important.
        Use the get_customer_info tool if user asks information or status about his order. The user must have provided already their email. This is important. If you do not know a user's email, simply ask a user for their email.
    """

    if enable_prompt_caching:
        system = [
            {
                "type": "text",
                "text": system_prompt,
                "cache_control": {"type": "ephemeral"},
            }
        ]
    else:
        system = system_prompt

    return system, [messages]
