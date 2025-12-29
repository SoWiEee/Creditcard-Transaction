export const validate = (schema) => {
    return async (request, reply) => {
        const { error } = schema.validate(request.body, { abortEarly: false });

        if (error) {
            const errors = error.details.map((detail) => detail.message);
            return reply.code(400).send({
                status: 'error',
                message: 'Invalid input data',
                errors,
            });
        }
    };
};
