// This file is auto-generated by @hey-api/openapi-ts

export interface BasicUserInformation {
  /**
   * Timestamp when the user was created
   */
  createdAt: string;
  /**
   * Email of the user
   */
  email: string;
  /**
   * First name of the user
   */
  firstName: string;
  iamUserId: string;
  id: number;
  /**
   * Last name of the user
   */
  lastName: string;
  organization: number;
  /**
   * Timestamp when the user was last updated
   */
  updatedAt: string;
  /**
   * User name
   */
  username: string;
}

export interface Category {
  Color: string;
  CreatedAt: string;
  Id: number;
  Label: string;
  LabelDescription: string;
  OrganizationId: number;
  UpdatedAt: string;
}

export interface CreateCategoryInputBodyReadable {
  /**
   * A URL to the JSON Schema for this object.
   */
  readonly $schema?: string;
  color: string;
  label: string;
  labelDescription: string;
}

export interface CreateCategoryInputBodyWritable {
  color: string;
  label: string;
  labelDescription: string;
}

export interface CreateCategoryOutputBodyReadable {
  /**
   * A URL to the JSON Schema for this object.
   */
  readonly $schema?: string;
  /**
   * ID of the created category
   */
  id: number;
}

export interface CreateCategoryOutputBodyWritable {
  /**
   * ID of the created category
   */
  id: number;
}

export interface CreateRequestInputBodyReadable {
  /**
   * A URL to the JSON Schema for this object.
   */
  readonly $schema?: string;
  /**
   * Address of the requester
   */
  address: string;
  /**
   * Identifier of the category
   */
  categoryId?: number;
  /**
   * Indicates if the request is closed
   */
  closed: boolean;
  /**
   * Email of the requester
   */
  email: string;
  /**
   * Name of the requester
   */
  name: string;
  /**
   * Subject of the request
   */
  subject: string;
  /**
   * Telephone number of the requester
   */
  telephone: string;
  /**
   * Description of the request
   */
  text: string;
}

export interface CreateRequestInputBodyWritable {
  /**
   * Address of the requester
   */
  address: string;
  /**
   * Identifier of the category
   */
  categoryId?: number;
  /**
   * Indicates if the request is closed
   */
  closed: boolean;
  /**
   * Email of the requester
   */
  email: string;
  /**
   * Name of the requester
   */
  name: string;
  /**
   * Subject of the request
   */
  subject: string;
  /**
   * Telephone number of the requester
   */
  telephone: string;
  /**
   * Description of the request
   */
  text: string;
}

export interface CreateRequestOutputBodyReadable {
  /**
   * A URL to the JSON Schema for this object.
   */
  readonly $schema?: string;
  /**
   * ID of the created category
   */
  id: number;
}

export interface CreateRequestOutputBodyWritable {
  /**
   * ID of the created category
   */
  id: number;
}

export interface CreateUserInputBodyReadable {
  /**
   * A URL to the JSON Schema for this object.
   */
  readonly $schema?: string;
  /**
   * Email of the user to be created
   */
  email: string;
  /**
   * First name of the user to be created
   */
  firstName: string;
  /**
   * Last name of the user to be created
   */
  lastName: string;
  /**
   * Initial password for the user to be set
   */
  password: string;
}

export interface CreateUserInputBodyWritable {
  /**
   * Email of the user to be created
   */
  email: string;
  /**
   * First name of the user to be created
   */
  firstName: string;
  /**
   * Last name of the user to be created
   */
  lastName: string;
  /**
   * Initial password for the user to be set
   */
  password: string;
}

export interface ErrorDetail {
  /**
   * Where the error occurred, e.g. 'body.items[3].tags' or 'path.thing-id'
   */
  location?: string;
  /**
   * Error message text
   */
  message?: string;
  /**
   * The value at the given location
   */
  value?: unknown;
}

export interface ErrorModelReadable {
  /**
   * A URL to the JSON Schema for this object.
   */
  readonly $schema?: string;
  /**
   * A human-readable explanation specific to this occurrence of the problem.
   */
  detail?: string;
  /**
   * Optional list of individual error details
   */
  errors?: Array<ErrorDetail>;
  /**
   * A URI reference that identifies the specific occurrence of the problem.
   */
  instance?: string;
  /**
   * HTTP status code
   */
  status?: number;
  /**
   * A short, human-readable summary of the problem type. This value should not change between occurrences of the error.
   */
  title?: string;
  /**
   * A URI reference to human-readable documentation for the error.
   */
  type?: string;
}

export interface ErrorModelWritable {
  /**
   * A human-readable explanation specific to this occurrence of the problem.
   */
  detail?: string;
  /**
   * Optional list of individual error details
   */
  errors?: Array<ErrorDetail>;
  /**
   * A URI reference that identifies the specific occurrence of the problem.
   */
  instance?: string;
  /**
   * HTTP status code
   */
  status?: number;
  /**
   * A short, human-readable summary of the problem type. This value should not change between occurrences of the error.
   */
  title?: string;
  /**
   * A URI reference to human-readable documentation for the error.
   */
  type?: string;
}

export interface GetCategoriesOutputBodyReadable {
  /**
   * A URL to the JSON Schema for this object.
   */
  readonly $schema?: string;
  /**
   * Array of categories
   */
  categories: Array<Category>;
}

export interface GetCategoriesOutputBodyWritable {
  /**
   * Array of categories
   */
  categories: Array<Category>;
}

export interface GetRequestsOutputBodyReadable {
  /**
   * A URL to the JSON Schema for this object.
   */
  readonly $schema?: string;
  /**
   * Number of requests to be returned
   */
  limit: number;
  /**
   * Cursor for pagination
   */
  nextCursor: number;
  /**
   * Array of requests
   */
  requests: Array<Request>;
}

export interface GetRequestsOutputBodyWritable {
  /**
   * Number of requests to be returned
   */
  limit: number;
  /**
   * Cursor for pagination
   */
  nextCursor: number;
  /**
   * Array of requests
   */
  requests: Array<Request>;
}

export interface GetRolesOutputBodyReadable {
  /**
   * A URL to the JSON Schema for this object.
   */
  readonly $schema?: string;
  /**
   * Array of roles that can be assigned to users
   */
  roles: Array<Role>;
}

export interface GetRolesOutputBodyWritable {
  /**
   * Array of roles that can be assigned to users
   */
  roles: Array<Role>;
}

export interface GetUserInfoOutputBodyReadable {
  /**
   * A URL to the JSON Schema for this object.
   */
  readonly $schema?: string;
  /**
   * Array of users with basic informations
   */
  users: Array<BasicUserInformation>;
}

export interface GetUserInfoOutputBodyWritable {
  /**
   * Array of users with basic informations
   */
  users: Array<BasicUserInformation>;
}

export interface GetUserOutputBodyReadable {
  /**
   * A URL to the JSON Schema for this object.
   */
  readonly $schema?: string;
  /**
   * Information about a user managed in given auth system
   */
  user: User;
}

export interface GetUserOutputBodyWritable {
  /**
   * Information about a user managed in given auth system
   */
  user: User;
}

export interface GetUsersOutputBodyReadable {
  /**
   * A URL to the JSON Schema for this object.
   */
  readonly $schema?: string;
  /**
   * Array of users managed in organization
   */
  users: Array<User>;
}

export interface GetUsersOutputBodyWritable {
  /**
   * Array of users managed in organization
   */
  users: Array<User>;
}

export interface PutUserRoleInputBody {
  /**
   * ID of the role to be assigned to the user
   */
  id: string;
  /**
   * Name of the role to be assigned to the user
   */
  name: string;
}

export interface Request {
  Address: string;
  CategoryId: number | null;
  Closed: boolean;
  CreatedAt: string;
  Email: string;
  Id: number;
  Name: string;
  OrganizationId: number;
  Raw: string;
  ReferenceId: string;
  Subject: string;
  Telephone: string;
  Text: string;
  UpdatedAt: string;
}

export interface Role {
  /**
   * ID of the role to be assigned to the user
   */
  id: string;
  /**
   * Name of the role to be assigned to the user
   */
  name: string;
}

export interface StatusOutputBodyReadable {
  /**
   * A URL to the JSON Schema for this object.
   */
  readonly $schema?: string;
  /**
   * Status of the API server
   */
  alive: boolean;
}

export interface StatusOutputBodyWritable {
  /**
   * Status of the API server
   */
  alive: boolean;
}

export interface UpdateCategoryInputBodyReadable {
  /**
   * A URL to the JSON Schema for this object.
   */
  readonly $schema?: string;
  color: string;
  label: string;
  labelDescription: string;
}

export interface UpdateCategoryInputBodyWritable {
  color: string;
  label: string;
  labelDescription: string;
}

export interface UpdateRequestInputBodyReadable {
  /**
   * A URL to the JSON Schema for this object.
   */
  readonly $schema?: string;
  /**
   * Address of the requester
   */
  address: string;
  /**
   * Identifier of the category
   */
  categoryId: number;
  /**
   * Indicates if the request is closed
   */
  closed: boolean;
  /**
   * Email of the requester
   */
  email: string;
  /**
   * Name of the requester
   */
  name: string;
  /**
   * Subject of the request
   */
  subject: string;
  /**
   * Telephone number of the requester
   */
  telephone: string;
  /**
   * Description of the request
   */
  text: string;
}

export interface UpdateRequestInputBodyWritable {
  /**
   * Address of the requester
   */
  address: string;
  /**
   * Identifier of the category
   */
  categoryId: number;
  /**
   * Indicates if the request is closed
   */
  closed: boolean;
  /**
   * Email of the requester
   */
  email: string;
  /**
   * Name of the requester
   */
  name: string;
  /**
   * Subject of the request
   */
  subject: string;
  /**
   * Telephone number of the requester
   */
  telephone: string;
  /**
   * Description of the request
   */
  text: string;
}

export interface UpdateUserInputBodyReadable {
  /**
   * A URL to the JSON Schema for this object.
   */
  readonly $schema?: string;
  /**
   * Email of the user
   */
  email: string;
  /**
   * Flag describing whether user email is verified or not
   */
  emailVerified: boolean;
  /**
   * Flag describing whether user is active or not
   */
  enabled: boolean;
  /**
   * First name of the user
   */
  firstName: string;
  /**
   * Last name of the user
   */
  lastName: string;
  /**
   * Array of actions that will be conducted after login
   */
  requiredActions: Array<"CONFIGURE_TOTP" | "UPDATE_PASSWORD" | "UPDATE_PROFILE" | "VERIFY_EMAIL">;
  /**
   * Flag describing whether TOTP was set or not
   */
  totp: boolean;
  /**
   * User name
   */
  username: string;
}

export interface UpdateUserInputBodyWritable {
  /**
   * Email of the user
   */
  email: string;
  /**
   * Flag describing whether user email is verified or not
   */
  emailVerified: boolean;
  /**
   * Flag describing whether user is active or not
   */
  enabled: boolean;
  /**
   * First name of the user
   */
  firstName: string;
  /**
   * Last name of the user
   */
  lastName: string;
  /**
   * Array of actions that will be conducted after login
   */
  requiredActions: Array<"CONFIGURE_TOTP" | "UPDATE_PASSWORD" | "UPDATE_PROFILE" | "VERIFY_EMAIL">;
  /**
   * Flag describing whether TOTP was set or not
   */
  totp: boolean;
  /**
   * User name
   */
  username: string;
}

export interface User {
  /**
   * Email of the user
   */
  email: string;
  /**
   * Flag describing whether user email is verified or not
   */
  emailVerified: boolean;
  /**
   * Flag describing whether user is active or not
   */
  enabled: boolean;
  /**
   * First name of the user
   */
  firstName: string;
  /**
   * User identifier within the authentication system
   */
  id: string;
  /**
   * Last name of the user
   */
  lastName: string;
  /**
   * Array of actions that will be conducted after login
   */
  requiredActions: Array<"CONFIGURE_TOTP" | "UPDATE_PASSWORD" | "UPDATE_PROFILE" | "VERIFY_EMAIL">;
  /**
   * Array of roles assigned to the user
   */
  roles?: Array<Role>;
  /**
   * Flag describing whether TOTP was set or not
   */
  totp: boolean;
  /**
   * User name
   */
  username: string;
}

export interface GetCategoriesData {
  body?: never;
  path: {
    /**
     * ID of the organization
     */
    organizationId: number;
  };
  query?: never;
  url: "/organizations/{organizationId}/categories";
}

export interface GetCategoriesErrors {
  /**
   * Error
   */
  default: ErrorModelReadable;
}

export type GetCategoriesError = GetCategoriesErrors[keyof GetCategoriesErrors];

export interface GetCategoriesResponses {
  /**
   * OK
   */
  200: GetCategoriesOutputBodyReadable;
}

export type GetCategoriesResponse = GetCategoriesResponses[keyof GetCategoriesResponses];

export interface CreateCategoryData {
  body: CreateCategoryInputBodyWritable;
  path: {
    /**
     * ID of the organization
     */
    organizationId: number;
  };
  query?: never;
  url: "/organizations/{organizationId}/categories";
}

export interface CreateCategoryErrors {
  /**
   * Error
   */
  default: ErrorModelReadable;
}

export type CreateCategoryError = CreateCategoryErrors[keyof CreateCategoryErrors];

export interface CreateCategoryResponses {
  /**
   * Created
   */
  201: CreateCategoryOutputBodyReadable;
}

export type CreateCategoryResponse = CreateCategoryResponses[keyof CreateCategoryResponses];

export interface DeleteCategoryData {
  body?: never;
  path: {
    /**
     * ID of the organization
     */
    organizationId: number;
    /**
     * ID of the category
     */
    categoryId: number;
  };
  query?: never;
  url: "/organizations/{organizationId}/categories/{categoryId}";
}

export interface DeleteCategoryErrors {
  /**
   * Error
   */
  default: ErrorModelReadable;
}

export type DeleteCategoryError = DeleteCategoryErrors[keyof DeleteCategoryErrors];

export interface DeleteCategoryResponses {
  /**
   * No Content
   */
  204: void;
}

export type DeleteCategoryResponse = DeleteCategoryResponses[keyof DeleteCategoryResponses];

export interface UpdateCategoryData {
  body: UpdateCategoryInputBodyWritable;
  path: {
    /**
     * ID of the organization
     */
    organizationId: number;
    /**
     * ID of the category
     */
    categoryId: number;
  };
  query?: never;
  url: "/organizations/{organizationId}/categories/{categoryId}";
}

export interface UpdateCategoryErrors {
  /**
   * Error
   */
  default: ErrorModelReadable;
}

export type UpdateCategoryError = UpdateCategoryErrors[keyof UpdateCategoryErrors];

export interface UpdateCategoryResponses {
  /**
   * No Content
   */
  204: void;
}

export type UpdateCategoryResponse = UpdateCategoryResponses[keyof UpdateCategoryResponses];

export interface GetRolesData {
  body?: never;
  path: {
    /**
     * ID of the organization
     */
    organizationId: number;
  };
  query?: never;
  url: "/organizations/{organizationId}/iam/roles";
}

export interface GetRolesErrors {
  /**
   * Error
   */
  default: ErrorModelReadable;
}

export type GetRolesError = GetRolesErrors[keyof GetRolesErrors];

export interface GetRolesResponses {
  /**
   * OK
   */
  200: GetRolesOutputBodyReadable;
}

export type GetRolesResponse = GetRolesResponses[keyof GetRolesResponses];

export interface GetUsersData {
  body?: never;
  path: {
    /**
     * ID of the organization
     */
    organizationId: number;
  };
  query?: {
    /**
     * Flag describing whether to sync users from auth system or not
     */
    sync?: boolean;
  };
  url: "/organizations/{organizationId}/iam/users";
}

export interface GetUsersErrors {
  /**
   * Error
   */
  default: ErrorModelReadable;
}

export type GetUsersError = GetUsersErrors[keyof GetUsersErrors];

export interface GetUsersResponses {
  /**
   * OK
   */
  200: GetUsersOutputBodyReadable;
}

export type GetUsersResponse = GetUsersResponses[keyof GetUsersResponses];

export interface CreateUserData {
  body: CreateUserInputBodyWritable;
  path: {
    /**
     * ID of the organization
     */
    organizationId: number;
  };
  query?: never;
  url: "/organizations/{organizationId}/iam/users";
}

export interface CreateUserErrors {
  /**
   * Error
   */
  default: ErrorModelReadable;
}

export type CreateUserError = CreateUserErrors[keyof CreateUserErrors];

export interface CreateUserResponses {
  /**
   * Created
   */
  201: unknown;
}

export interface DeleteUserData {
  body?: never;
  path: {
    /**
     * ID of the organization
     */
    organizationId: number;
    /**
     * IAM id of the user to be deleted
     */
    iamUserId: string;
  };
  query?: never;
  url: "/organizations/{organizationId}/iam/users/{iamUserId}";
}

export interface DeleteUserErrors {
  /**
   * Error
   */
  default: ErrorModelReadable;
}

export type DeleteUserError = DeleteUserErrors[keyof DeleteUserErrors];

export interface DeleteUserResponses {
  /**
   * No Content
   */
  204: void;
}

export type DeleteUserResponse = DeleteUserResponses[keyof DeleteUserResponses];

export interface GetUserData {
  body?: never;
  path: {
    /**
     * ID of the organization
     */
    organizationId: number;
    /**
     * IAM id of a user
     */
    iamUserId: string;
  };
  query?: never;
  url: "/organizations/{organizationId}/iam/users/{iamUserId}";
}

export interface GetUserErrors {
  /**
   * Error
   */
  default: ErrorModelReadable;
}

export type GetUserError = GetUserErrors[keyof GetUserErrors];

export interface GetUserResponses {
  /**
   * OK
   */
  200: GetUserOutputBodyReadable;
}

export type GetUserResponse = GetUserResponses[keyof GetUserResponses];

export interface UpdateUserData {
  body: UpdateUserInputBodyWritable;
  path: {
    /**
     * ID of the organization
     */
    organizationId: number;
    /**
     * IAM id of the user to be deleted
     */
    iamUserId: string;
  };
  query?: never;
  url: "/organizations/{organizationId}/iam/users/{iamUserId}";
}

export interface UpdateUserErrors {
  /**
   * Error
   */
  default: ErrorModelReadable;
}

export type UpdateUserError = UpdateUserErrors[keyof UpdateUserErrors];

export interface UpdateUserResponses {
  /**
   * No Content
   */
  204: void;
}

export type UpdateUserResponse = UpdateUserResponses[keyof UpdateUserResponses];

export interface AssignUserRolesData {
  /**
   * Array of role representations
   */
  body: Array<PutUserRoleInputBody>;
  path: {
    /**
     * ID of the organization
     */
    organizationId: number;
    /**
     * ID of the user to be deleted
     */
    iamUserId: string;
  };
  query?: never;
  url: "/organizations/{organizationId}/iam/users/{iamUserId}/roles";
}

export interface AssignUserRolesErrors {
  /**
   * Error
   */
  default: ErrorModelReadable;
}

export type AssignUserRolesError = AssignUserRolesErrors[keyof AssignUserRolesErrors];

export interface AssignUserRolesResponses {
  /**
   * No Content
   */
  204: void;
}

export type AssignUserRolesResponse = AssignUserRolesResponses[keyof AssignUserRolesResponses];

export interface GetRequestsData {
  body?: never;
  path: {
    /**
     * ID of the organization
     */
    organizationId: number;
  };
  query: {
    /**
     * Flag describing whether to get also closed requests or not
     */
    getClosed?: boolean;
    /**
     * Number of requests to be returned
     */
    pageSize: number;
    /**
     * Cursor for pagination
     */
    cursor?: number;
  };
  url: "/organizations/{organizationId}/requests";
}

export interface GetRequestsErrors {
  /**
   * Error
   */
  default: ErrorModelReadable;
}

export type GetRequestsError = GetRequestsErrors[keyof GetRequestsErrors];

export interface GetRequestsResponses {
  /**
   * OK
   */
  200: GetRequestsOutputBodyReadable;
}

export type GetRequestsResponse = GetRequestsResponses[keyof GetRequestsResponses];

export interface CreateRequestData {
  body: CreateRequestInputBodyWritable;
  path: {
    /**
     * ID of the organization
     */
    organizationId: number;
  };
  query?: never;
  url: "/organizations/{organizationId}/requests";
}

export interface CreateRequestErrors {
  /**
   * Error
   */
  default: ErrorModelReadable;
}

export type CreateRequestError = CreateRequestErrors[keyof CreateRequestErrors];

export interface CreateRequestResponses {
  /**
   * Created
   */
  201: CreateRequestOutputBodyReadable;
}

export type CreateRequestResponse = CreateRequestResponses[keyof CreateRequestResponses];

export interface DeleteRequestData {
  body?: never;
  path: {
    /**
     * ID of the organization
     */
    organizationId: number;
    /**
     * ID of the request
     */
    requestId: number;
  };
  query?: never;
  url: "/organizations/{organizationId}/requests/{requestId}";
}

export interface DeleteRequestErrors {
  /**
   * Error
   */
  default: ErrorModelReadable;
}

export type DeleteRequestError = DeleteRequestErrors[keyof DeleteRequestErrors];

export interface DeleteRequestResponses {
  /**
   * No Content
   */
  204: void;
}

export type DeleteRequestResponse = DeleteRequestResponses[keyof DeleteRequestResponses];

export interface UpdateRequestData {
  body: UpdateRequestInputBodyWritable;
  path: {
    /**
     * ID of the organization
     */
    organizationId: number;
    /**
     * ID of the request
     */
    requestId: number;
  };
  query?: never;
  url: "/organizations/{organizationId}/requests/{requestId}";
}

export interface UpdateRequestErrors {
  /**
   * Error
   */
  default: ErrorModelReadable;
}

export type UpdateRequestError = UpdateRequestErrors[keyof UpdateRequestErrors];

export interface UpdateRequestResponses {
  /**
   * No Content
   */
  204: void;
}

export type UpdateRequestResponse = UpdateRequestResponses[keyof UpdateRequestResponses];

export interface GetBasicUserInformationData {
  body?: never;
  path: {
    /**
     * ID of the organization
     */
    organizationId: number;
  };
  query?: never;
  url: "/organizations/{organizationId}/users";
}

export interface GetBasicUserInformationErrors {
  /**
   * Error
   */
  default: ErrorModelReadable;
}

export type GetBasicUserInformationError = GetBasicUserInformationErrors[keyof GetBasicUserInformationErrors];

export interface GetBasicUserInformationResponses {
  /**
   * OK
   */
  200: GetUserInfoOutputBodyReadable;
}

export type GetBasicUserInformationResponse = GetBasicUserInformationResponses[keyof GetBasicUserInformationResponses];

export interface ServerStatusData {
  body?: never;
  path?: never;
  query?: never;
  url: "/status";
}

export interface ServerStatusErrors {
  /**
   * Error
   */
  default: ErrorModelReadable;
}

export type ServerStatusError = ServerStatusErrors[keyof ServerStatusErrors];

export interface ServerStatusResponses {
  /**
   * OK
   */
  200: StatusOutputBodyReadable;
}

export type ServerStatusResponse = ServerStatusResponses[keyof ServerStatusResponses];

export interface ClientOptions {
  baseURL: "http://localhost:8000/api" | (string & {});
}
