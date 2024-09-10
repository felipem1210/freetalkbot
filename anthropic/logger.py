import logging


# Función para obtener un logger
def get_logger():
    logger = logging.getLogger("my_app_logger")
    if not logger.hasHandlers():
        # Configuramos el logger si no tiene ya un manejador
        handler = logging.StreamHandler()
        formatter = logging.Formatter("%(asctime)s - %(levelname)s - %(message)s")
        handler.setFormatter(formatter)
        logger.addHandler(handler)
        logger.setLevel(logging.INFO)
    return logger
